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
