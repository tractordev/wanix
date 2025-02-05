package fs_test

import (
	"errors"
	"testing"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

func TestMkdir(t *testing.T) {
	fsys := fskit.MemFS{}
	err := fs.Mkdir(fsys, "test", 0755)
	if err != nil {
		t.Fatal(err)
	}

	dir, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(dir) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(dir))
	}
}

func TestMkdirNoParent(t *testing.T) {
	fsys := fskit.MemFS{}
	err := fs.Mkdir(fsys, "test/test2", 0755)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestMkdirAllOnFsysWithoutMkdirAll(t *testing.T) {
	fsys := fskit.MemFS{}
	err := fs.MkdirAll(fsys, "test/test2/test3", 0755)
	if err != nil {
		t.Fatal(err)
	}

	dir, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(dir) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(dir))
	}

	dir, err = fs.ReadDir(fsys, "test/test2")
	if err != nil {
		t.Fatal(err)
	}
	if len(dir) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(dir))
	}
}

func TestMkdirAllOnLeafFsysWithMkdir(t *testing.T) {
	fsys := fskit.MapFS{
		"sub": fskit.MemFS{
			"file": fskit.RawNode([]byte("file")),
		},
	}
	err := fs.MkdirAll(fsys, "sub/dir1/dir2", 0755)
	if err != nil {
		t.Fatal(err)
	}

	dir, err := fs.ReadDir(fsys, "sub")
	if err != nil {
		t.Fatal(err)
	}
	if len(dir) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(dir))
	}

	dir, err = fs.ReadDir(fsys, "sub/dir1")
	if err != nil {
		t.Fatal(err)
	}
	if len(dir) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(dir))
	}
}
