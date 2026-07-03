package allocfs

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/vfs"
)

type testResource struct {
	id string
	fskit.MapFS
}

func (r *testResource) ID() string { return r.id }

func readText(t *testing.T, f fs.File) string {
	t.Helper()
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

func mustOpen(t *testing.T, fsys fs.FS, name string) fs.File {
	t.Helper()
	f, err := fs.OpenContext(context.Background(), fsys, name)
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	return f
}

func TestNewAllocatesResource(t *testing.T) {
	fsys := New(func(ctx context.Context, id string, opts map[string]string) (fs.FS, error) {
		return &testResource{
			id: id,
			MapFS: fskit.MapFS{
				"data": fskit.RawNode([]byte("ok\n"), 0555),
			},
		}, nil
	})

	id := writeInvoke(t, mustOpen(t, fsys, "new"))
	if id != "1" {
		t.Fatalf("expected id 1, got %q", id)
	}

	data := strings.TrimSpace(readText(t, mustOpen(t, fsys, id+"/data")))
	if data != "ok" {
		t.Fatalf("expected data ok, got %q", data)
	}

	id2 := writeInvoke(t, mustOpen(t, fsys, "new"))
	if id2 != "2" {
		t.Fatalf("expected id 2, got %q", id2)
	}
}

func writeInvoke(t *testing.T, f fs.File) string {
	t.Helper()
	defer f.Close()
	if _, err := fs.Write(f, []byte("\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	return strings.TrimSpace(readText(t, f))
}

func TestNewReturnsBoundPath(t *testing.T) {
	fsys := New(func(ctx context.Context, id string, opts map[string]string) (fs.FS, error) {
		return &testResource{id: id}, nil
	})

	ns := vfs.New(context.Background())
	if err := ns.Bind(fsys, ".", "#fsys"); err != nil {
		t.Fatal(err)
	}

	f, err := fs.OpenFile(ns, "#fsys/new", os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	got := writeInvoke(t, f)
	if got != "#fsys/1" {
		t.Fatalf("expected #fsys/1, got %q", got)
	}
}

func TestLookup(t *testing.T) {
	fsys := New(func(ctx context.Context, id string, opts map[string]string) (fs.FS, error) {
		return &testResource{id: id}, nil
	})
	id := writeInvoke(t, mustOpen(t, fsys, "new"))
	if _, err := fsys.Lookup(id); err != nil {
		t.Fatalf("lookup %s: %v", id, err)
	}
	if _, err := fsys.Lookup("missing"); err == nil {
		t.Fatalf("expected lookup error")
	}
}

func TestSymlinkToResource(t *testing.T) {
	fsys := New(func(ctx context.Context, id string, opts map[string]string) (fs.FS, error) {
		return &testResource{
			id: id,
			MapFS: fskit.MapFS{
				"data": fskit.RawNode([]byte("via-link\n"), 0644),
			},
		}, nil
	})

	id := writeInvoke(t, mustOpen(t, fsys, "new"))
	if err := fs.Symlink(fsys, id, "alias"); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	target, err := fs.Readlink(fsys, "alias")
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != id {
		t.Fatalf("readlink = %q, want %q", target, id)
	}

	data := strings.TrimSpace(readText(t, mustOpen(t, fsys, "alias/data")))
	if data != "via-link" {
		t.Fatalf("expected via-link, got %q", data)
	}

	fi, err := fs.Lstat(fsys, "alias")
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if fi.Mode()&fs.ModeSymlink == 0 {
		t.Fatal("expected symlink mode")
	}
}

func TestSymlinkRejectsMissingResource(t *testing.T) {
	fsys := New(func(ctx context.Context, id string, opts map[string]string) (fs.FS, error) {
		return &testResource{id: id}, nil
	})
	if err := fs.Symlink(fsys, "missing", "alias"); err == nil {
		t.Fatal("expected symlink error for missing resource")
	}
}
