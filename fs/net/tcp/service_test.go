//go:build !js || !wasm

package tcp

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tractor.dev/wanix/fs"
)

func readText(t *testing.T, f fs.File) string {
	t.Helper()
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

func writeText(t *testing.T, fsys fs.FS, name, data string) {
	t.Helper()
	if err := fs.WriteFile(fsys, name, []byte(data), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestServiceNewAllocatesConn(t *testing.T) {
	s := New()
	f, err := s.Open("new")
	if err != nil {
		t.Fatalf("open new: %v", err)
	}
	id := strings.TrimSpace(readText(t, f))
	if id == "" {
		t.Fatalf("expected id, got empty")
	}
	// status should exist
	sf, err := s.Open(id + "/status")
	if err != nil {
		t.Fatalf("open status: %v", err)
	}
	_ = readText(t, sf)
}

func TestDialTCPAndDataEcho(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	// simple echo server
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(c)
		}
	}()

	s := New()
	id := strings.TrimSpace(readText(t, mustOpen(t, s, "new")))
	writeText(t, s, id+"/ctl", "dial "+ln.Addr().String()+"\n")

	df, err := s.Open(id + "/data")
	if err != nil {
		t.Fatalf("open data: %v", err)
	}
	defer df.Close()

	msg := []byte("hello")
	if _, err := fs.Write(df, msg); err != nil {
		t.Fatalf("write data: %v", err)
	}
	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(df, buf); err != nil {
		t.Fatalf("read data: %v", err)
	}
	if string(buf) != string(msg) {
		t.Fatalf("echo mismatch: got %q want %q", string(buf), string(msg))
	}
}

func TestAnnounceListenAcceptAllocatesConnectedConn(t *testing.T) {
	s := New()
	listenerID := strings.TrimSpace(readText(t, mustOpen(t, s, "new")))

	writeText(t, s, listenerID+"/ctl", "announce 127.0.0.1:0\n")
	local := strings.TrimSpace(readText(t, mustOpen(t, s, listenerID+"/local")))
	if local == "" {
		t.Fatalf("expected local addr")
	}

	// connect from the outside
	client, err := net.Dial("tcp", local)
	if err != nil {
		t.Fatalf("dial local: %v", err)
	}
	defer client.Close()

	// accept via listen file
	acceptedID := strings.TrimSpace(readText(t, mustOpen(t, s, listenerID+"/listen")))
	if acceptedID == "" || acceptedID == listenerID {
		t.Fatalf("bad accepted id: %q", acceptedID)
	}

	// prove connection works: send from client, read from accepted/data
	ad, err := s.Open(acceptedID + "/data")
	if err != nil {
		t.Fatalf("open accepted data: %v", err)
	}
	defer ad.Close()

	want := []byte("ping")
	if _, err := client.Write(want); err != nil {
		t.Fatalf("client write: %v", err)
	}
	got := make([]byte, len(want))
	if _, err := io.ReadFull(ad, got); err != nil {
		t.Fatalf("accepted read: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("accepted read mismatch: got %q want %q", got, want)
	}
}

func TestDialUnixPath(t *testing.T) {
	tmp := t.TempDir()
	sock := filepath.Join(tmp, "wanix-test.sock")

	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(sock)
	}()

	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = io.Copy(c, c)
	}()

	s := New()
	id := strings.TrimSpace(readText(t, mustOpen(t, s, "new")))
	writeText(t, s, id+"/ctl", "dial "+sock+"\n")

	df, err := s.Open(id + "/data")
	if err != nil {
		t.Fatalf("open data: %v", err)
	}
	defer df.Close()

	if _, err := fs.Write(df, []byte("x")); err != nil {
		t.Fatalf("write: %v", err)
	}
	b := make([]byte, 1)
	if _, err := io.ReadFull(df, b); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(b) != "x" {
		t.Fatalf("echo mismatch: %q", b)
	}
}

func TestHangupClosesData(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = io.Copy(c, c)
	}()

	s := New()
	id := strings.TrimSpace(readText(t, mustOpen(t, s, "new")))
	writeText(t, s, id+"/ctl", "dial "+ln.Addr().String()+"\n")

	writeText(t, s, id+"/ctl", "hangup\n")
	_, err = s.Open(id + "/data")
	if err == nil {
		t.Fatalf("expected error opening data after hangup")
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("expected ErrPermission, got %v", err)
	}
}

func mustOpen(t *testing.T, fsys fs.FS, name string) fs.File {
	t.Helper()
	f, err := fs.OpenContext(context.Background(), fsys, name)
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	return f
}

