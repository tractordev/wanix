package p9kit

import (
	"context"
	"encoding/binary"
	"net"
	"sync"
	"testing"

	"github.com/hugelgupf/p9/p9"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/vfs"
)

// 9P2000.L T-message types counted by the wire-cost baselines.
const (
	msgTlopen   = 12
	msgTgetattr = 24
	msgTreaddir = 40
	msgTfsync   = 50
	msgTwalk    = 110
	msgTclunk   = 120
)

// countingConn tallies the 9P T-messages the client writes. The p9
// client emits one message across several Write calls, so frames are
// reassembled from the byte stream by their size[4] header before the
// type byte at offset 4 is counted.
type countingConn struct {
	net.Conn
	mu     sync.Mutex
	buf    []byte
	counts map[uint8]int
}

func (cc *countingConn) Write(p []byte) (int, error) {
	cc.mu.Lock()
	cc.buf = append(cc.buf, p...)
	for len(cc.buf) >= 4 {
		size := binary.LittleEndian.Uint32(cc.buf)
		if size < 7 || uint32(len(cc.buf)) < size {
			break
		}
		cc.counts[cc.buf[4]]++
		cc.buf = cc.buf[size:]
	}
	cc.mu.Unlock()
	return cc.Conn.Write(p)
}

// take returns the counts accumulated since the last take and resets.
func (cc *countingConn) take() map[uint8]int {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	got := cc.counts
	cc.counts = map[uint8]int{}
	return got
}

func countingSetup(t *testing.T, backend fs.FS) (fs.FS, *countingConn, func()) {
	t.Helper()
	a, b := net.Pipe()
	srv := p9.NewServer(Attacher(backend))
	done := make(chan error, 1)
	go func() {
		done <- srv.Handle(a, a)
	}()
	cc := &countingConn{Conn: b, counts: map[uint8]int{}}
	fsys, err := ClientFS(cc, "")
	if err != nil {
		t.Fatalf("ClientFS: %v", err)
	}
	cleanup := func() {
		b.Close()
		a.Close()
		<-done
	}
	return fsys, cc, cleanup
}

func expectCounts(t *testing.T, phase string, got map[uint8]int, want map[uint8]int) {
	t.Helper()
	names := map[uint8]string{
		msgTlopen: "Tlopen", msgTgetattr: "Tgetattr", msgTreaddir: "Treaddir",
		msgTfsync: "Tfsync", msgTwalk: "Twalk", msgTclunk: "Tclunk",
	}
	for typ, n := range want {
		if got[typ] != n {
			t.Errorf("%s: %s = %d, want %d", phase, names[typ], got[typ], n)
		}
	}
}

// TestServerReaddirWireCost pins the wire cost of the path the rc
// shell actually drives: a p9kit SERVER fronting a namespace whose
// directory is a p9kit CLIENT mount (the `3ds`-style relay mount).
// This composition — not FS.ReadDir in isolation — is what a gojs
// task's `ls` hits, and it was the real "300 stats for one ls"
// amplifier: server.Readdir used to stat EVERY entry through the mount
// (walk+getattr+clunk each) to fill in dirent QIDs, on top of the
// listing itself.
//
// The fix builds each dirent QID from the DirEntry alone (type from
// the .L dirent, path from a string hash), so a k-entry directory
// costs one client listing and ZERO per-entry round-trips. If someone
// reintroduces a per-entry stat here, Tgetattr goes back above zero
// and this fails.
func TestServerReaddirWireCost(t *testing.T) {
	backend := fskit.MapFS{
		"f1": fskit.RawNode([]byte("x")), "f2": fskit.RawNode([]byte("x")),
		"f3": fskit.RawNode([]byte("x")), "f4": fskit.RawNode([]byte("x")),
		"f5":        fskit.RawNode([]byte("x")),
		"sub/inner": fskit.RawNode([]byte("x")),
		"sub2/deep": fskit.RawNode([]byte("x")),
	}
	fsys, cc, cleanup := countingSetup(t, backend)
	defer cleanup()
	cc.take() // discard version/attach setup traffic

	// The server p9file that fronts the mounted client fs to a task.
	srv := &p9file{path: ".", fsys: fsys}
	dents, err := srv.Readdir(0, 100)
	if err != nil {
		t.Fatalf("server Readdir: %v", err)
	}
	if len(dents) != 7 {
		t.Fatalf("dents = %d, want 7", len(dents))
	}

	// One client listing, no per-entry stats: walk+lopen+readdir+clunk.
	got := cc.take()
	if got[msgTgetattr] != 0 {
		t.Errorf("server Readdir issued %d Tgetattr over the mount — per-entry stat is back (the storm)", got[msgTgetattr])
	}
	total := 0
	for _, n := range got {
		total += n
	}
	if total > 6 {
		t.Errorf("server Readdir wire cost = %d messages, want <=6 (one listing, no per-entry traffic); got %v", total, got)
	}

	// Dirents must still be correctly typed from the DirEntry alone.
	dt := map[string]p9.QIDType{}
	for _, d := range dents {
		dt[d.Name] = d.Type
	}
	if dt["sub"] != p9.QIDType(4) { // DT_DIR
		t.Errorf("sub dirent type = %d, want 4 (DT_DIR)", dt["sub"])
	}
	if dt["f1"] != p9.QIDType(8) { // DT_REG
		t.Errorf("f1 dirent type = %d, want 8 (DT_REG)", dt["f1"])
	}
}

// TestRemoteFileReadDirLabelsEntries covers the correctness half of
// the listing fix: remoteFile.ReadDir used to stat the DIRECTORY's own
// fid for every entry, so each entry reported the directory's
// attributes — a plain file claimed IsDir()==true. Entries are now
// typed by their own dirent qid, and Info() walks to the entry itself.
func TestRemoteFileReadDirLabelsEntries(t *testing.T) {
	backend := fskit.MapFS{"sub/inner": fskit.RawNode([]byte("hello"))}
	fsys, cc, cleanup := countingSetup(t, backend)
	defer cleanup()

	f, err := fs.OpenContext(context.Background(), fsys, "sub")
	if err != nil {
		t.Fatalf("OpenContext: %v", err)
	}
	defer f.Close()
	entries, err := f.(fs.ReadDirFile).ReadDir(-1)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "inner" {
		t.Fatalf("entries = %v", entries)
	}
	if entries[0].IsDir() {
		t.Fatalf("inner is a plain file but IsDir() = true (the pre-fix mislabel)")
	}
	if entries[0].Type() != 0 {
		t.Fatalf("Type() = %v, want 0 (regular file)", entries[0].Type())
	}

	// Info() is lazy: it walks to the entry itself and stats it there,
	// on demand, rather than mislabeling it up front.
	cc.take()
	fi, err := entries[0].Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if fi.IsDir() || fi.Size() != 5 {
		t.Fatalf("Info = dir:%v size:%d, want plain 5-byte file", fi.IsDir(), fi.Size())
	}
	expectCounts(t, "lazy Info", cc.take(), map[uint8]int{
		msgTwalk:    1,
		msgTgetattr: 1,
		msgTclunk:   1,
	})
}

// TestNSReadDirWireCost drives a directory read the way api/readdir.go
// does for a task's `ls`: through a vfs.NS namespace with a 9P client
// mount bound into it. Reading a k-entry directory must cost one
// listing, not k stats: the namespace keeps entries lazy (vfs) and the
// client builds them from dirents, so no layer re-stats each entry.
func TestNSReadDirWireCost(t *testing.T) {
	backend := fskit.MapFS{}
	subs := []string{"audio", "camera", "display", "input", "ir", "led", "nfc", "power", "sd", "system"}
	for _, s := range subs {
		backend["dev/"+s+"/a"] = fskit.RawNode([]byte("x"))
	}
	client, cc, cleanup := countingSetup(t, backend)
	defer cleanup()

	ns := vfs.New(context.Background())
	if err := ns.Bind(client, ".", ".", vfs.BindReplace); err != nil {
		t.Fatalf("bind: %v", err)
	}
	cc.take() // discard setup + bind traffic

	entries, err := fs.ReadDir(ns, "dev")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 10 {
		t.Fatalf("entries = %d, want 10", len(entries))
	}
	got := cc.take()
	if got[msgTgetattr] > 1 {
		t.Errorf("namespace ReadDir issued %d getattrs — some layer is statting every entry again", got[msgTgetattr])
	}
}
