package fskit

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"tractor.dev/wanix/fs"
)

func TestMemFSCreate(t *testing.T) {
	m := MemFS{
		"hello": RawNode([]byte("hello, world\n")),
	}

	// check for failure if file already exists
	if _, err := fs.Create(m, "hello"); err == nil {
		t.Fatal("expected error that file already exists")
	}

	// check for success
	if _, err := fs.Create(m, "fortune"); err != nil {
		t.Fatal(err)
	}
	if err := fstest.TestFS(m, "fortune"); err != nil {
		t.Fatal(err)
	}
}

func TestMemFSMkdir(t *testing.T) {
	m := MemFS{
		"hello": RawNode([]byte("hello, world\n")),
	}

	// check for failure if file already exists
	if err := fs.Mkdir(m, "hello", 0755); err == nil {
		t.Fatal("expected error")
	}

	// check for failure if parent directory does not exist
	if err := fs.Mkdir(m, "foo/bar", 0755); err == nil {
		t.Fatal("expected error")
	}

	// check for success
	if err := fs.Mkdir(m, "fortune", 0755); err != nil {
		t.Fatal(err)
	}
	if err := fstest.TestFS(m, "fortune"); err != nil {
		t.Fatal(err)
	}
}

func TestMemFSChtimes(t *testing.T) {
	m := MemFS{
		"hello": RawNode([]byte("hello, world\n")),
	}

	// check for failure if file does not exist
	if err := fs.Chtimes(m, "foo/bar", time.Now(), time.Now()); err == nil {
		t.Fatal("expected error")
	}

	// check for success
	atime := time.Now()
	mtime := time.Now()
	if err := fs.Chtimes(m, "hello", atime, mtime); err != nil {
		t.Fatal(err)
	}
	info, _ := fs.Stat(m, "hello")
	got := info.ModTime()
	if mtime != got {
		t.Errorf("FS FileInfo.ModTime want:\n%s\ngot:\n%s\n", mtime, got)
	}
}

func TestMemFSChmod(t *testing.T) {
	m := MemFS{
		"hello": RawNode([]byte("hello, world\n"), fs.FileMode(0666)),
	}

	// check for failure if file does not exist
	if err := fs.Chmod(m, "foo/bar", 0755); err == nil {
		t.Fatal("expected error")
	}

	// check for success
	if err := fs.Chmod(m, "hello", 0777); err != nil {
		t.Fatal(err)
	}
	info, _ := fs.Stat(m, "hello")
	want := fs.FileMode(0777)
	got := info.Mode()
	if want != got {
		t.Errorf("FS FileInfo.Mode want:\n%s\ngot:\n%s\n", want, got)
	}
}

func TestMemFSRemove(t *testing.T) {
	m := MemFS{
		"hello":   RawNode([]byte("hello, world\n")),
		"foo/bar": RawNode([]byte("foobar\n")),
	}

	// check for failure if file does not exist
	if err := fs.Remove(m, "unknown"); err == nil {
		t.Fatal("no failure for non-existent file")
	}

	// check for failure if directory is not empty
	// if err := fs.Remove(m, "foo"); err == nil {
	// 	t.Fatal("no failure for non-empty directory")
	// }

	// check for success
	if err := fs.Remove(m, "hello"); err != nil {
		t.Fatal(err)
	}
	ok, err := fs.Exists(m, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected file to be removed")
	}
}

func TestMemFSRename(t *testing.T) {
	m := MemFS{
		"hello": RawNode([]byte("hello, world\n")),
	}

	// check for failure if oldfile does not exist
	if err := fs.Rename(m, "foo/bar", "hello"); err == nil {
		t.Fatal("expected error that old file does not exist")
	}

	// check for failure if newfile parent directory does not exist
	if err := fs.Rename(m, "hello", "foo/hello"); err == nil {
		t.Fatal("expected error that parent directory does not exist")
	}

	// check for success
	if err := fs.Rename(m, "hello", "fortune"); err != nil {
		t.Fatal(err)
	}
	if err := fstest.TestFS(m, "fortune"); err != nil {
		t.Fatal(err)
	}
}

func TestMemFS(t *testing.T) {
	m := MemFS{
		"hello":             RawNode([]byte("hello, world\n")),
		"fortune/k/ken.txt": RawNode([]byte("If a program is too slow, it must have a loop.\n")),
	}
	if err := fstest.TestFS(m, "hello", "fortune", "fortune/k", "fortune/k/ken.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestMemFSChmodDot(t *testing.T) {
	m := MemFS{
		"a/b.txt": RawNode(fs.FileMode(0666)),
		".":       RawNode(fs.FileMode(0777 | fs.ModeDir)),
	}
	buf := new(strings.Builder)
	fs.WalkDir(m, ".", func(path string, d fs.DirEntry, _ error) error {
		fi, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(buf, "%s: %v\n", path, fi.Mode())
		return nil
	})
	want := `
.: drwxrwxrwx
a: drwxr-xr-x
a/b.txt: -rw-rw-rw-
`[1:]
	got := buf.String()
	if want != got {
		t.Errorf("MapFS modes want:\n%s\ngot:\n%s\n", want, got)
	}
}

func TestMemFSFileInfoName(t *testing.T) {
	m := MemFS{
		"path/to/b.txt": RawNode(),
	}
	info, _ := fs.Stat(m, "path/to/b.txt")
	want := "b.txt"
	got := info.Name()
	if want != got {
		t.Errorf("MapFS FileInfo.Name want:\n%s\ngot:\n%s\n", want, got)
	}
}
