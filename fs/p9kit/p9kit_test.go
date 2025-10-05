package p9kit

import (
	"io"
	"io/fs"
	"net"
	"testing"

	"github.com/hugelgupf/p9/p9"
	"tractor.dev/wanix/fs/fskit"
)

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error {
	return nil
}

func TestIntegration(t *testing.T) {
	backend := fskit.MapFS{
		"foo1":     fskit.RawNode([]byte("bar1")),
		"foo2":     fskit.RawNode([]byte("bar2")),
		"sub/foo3": fskit.RawNode([]byte("bar3")),
	}

	a, b := net.Pipe()
	srv := p9.NewServer(Attacher(backend)) //, p9.WithServerLogger(ulog.Log))
	// var bufIn, bufOut bytes.Buffer
	// defer func() {
	// 	fmt.Println("in:", bufIn.Bytes())
	// 	fmt.Println("out:", bufOut.Bytes())
	// }()
	go func() {
		// out := &nopCloser{io.MultiWriter(a, &bufOut)}
		// in := io.NopCloser(io.TeeReader(a, &bufIn))
		if err := srv.Handle(a, a); err != nil {
			t.Errorf("server.Handle: %v", err)
		}
	}()

	// conn, err := net.Dial("tcp", "localhost:3333")
	// if err != nil {
	// 	log.Fatal("conn:", err)
	// }
	// defer conn.Close()

	fsys, err := ClientFS(b, "")
	if err != nil {
		t.Fatalf("client.ClientFS: %v", err)
	}

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatalf("fs.ReadDir: %T %v", err, err)
	}
	if len(entries) != 3 {
		t.Fatalf("fs.ReadDir: expected 3 entries, got %d", len(entries))
	}

	if entries[0].Name() != "foo1" {
		t.Fatalf("fs.ReadDir: expected foo1, got %s", entries[0].Name())
	}
}

// duplicateReadDirFile is a mock that returns duplicate entries to test infinite loop protection
type duplicateReadDirFile struct {
	entries []fs.DirEntry
	calls   int
}

func (d *duplicateReadDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	d.calls++

	// Always return the first entry to simulate a buggy implementation
	if len(d.entries) > 0 && n > 0 {
		return d.entries[:1], nil
	}

	// After many calls, return EOF to prevent true infinite loop in test
	if d.calls > 100 {
		return nil, io.EOF
	}

	return d.entries, nil
}

func (d *duplicateReadDirFile) Stat() (fs.FileInfo, error) {
	return fskit.Entry("test-dir", fs.ModeDir|0755), nil
}

func (d *duplicateReadDirFile) Read([]byte) (int, error) {
	return 0, fs.ErrInvalid
}

func (d *duplicateReadDirFile) Close() error {
	return nil
}

func TestReaddir_DuplicateDetection(t *testing.T) {
	// Create mock entries
	entries := []fs.DirEntry{
		fskit.Entry("file1.txt", 0644),
		fskit.Entry("file2.txt", 0644),
	}

	// Create a mock file that returns duplicates
	mockFile := &duplicateReadDirFile{entries: entries}

	// Create a filesystem with the files so stat works
	backend := fskit.MapFS{
		"file1.txt": fskit.RawNode([]byte("content1")),
		"file2.txt": fskit.RawNode([]byte("content2")),
	}

	// Create p9file wrapper
	p9f := &p9file{
		path: ".",
		file: mockFile,
		fsys: backend,
	}

	// Call Readdir - should detect duplicates and break the loop
	dents, err := p9f.Readdir(0, 10)
	if err != nil {
		t.Fatalf("Readdir failed: %v", err)
	}

	// Should only get one entry (the duplicate detection should prevent more)
	if len(dents) != 1 {
		t.Errorf("Expected 1 entry due to duplicate detection, got %d", len(dents))
	}

	if len(dents) > 0 && dents[0].Name != "file1.txt" {
		t.Errorf("Expected first entry to be 'file1.txt', got %q", dents[0].Name)
	}

	// Verify that the mock was called multiple times but stopped due to duplicate detection
	if mockFile.calls < 2 {
		t.Errorf("Expected multiple calls to ReadDir, got %d", mockFile.calls)
	}

	// Should not have made 100+ calls (which would indicate infinite loop)
	if mockFile.calls >= 100 {
		t.Errorf("Too many calls to ReadDir (%d), duplicate detection may not be working", mockFile.calls)
	}
}
