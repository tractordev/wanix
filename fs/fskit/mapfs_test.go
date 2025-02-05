package fskit

import (
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestMapFS(t *testing.T) {
	m := MapFS{
		"hello":             RawNode([]byte("hello, world\n")),
		"fortune/k/ken.txt": RawNode([]byte("If a program is too slow, it must have a loop.\n")),
	}
	if err := fstest.TestFS(m, "hello", "fortune", "fortune/k", "fortune/k/ken.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestMapFSSubdirs(t *testing.T) {
	subdir := MapFS{
		"hello":             RawNode([]byte("hello, world\n")),
		"fortune/k/ken.txt": RawNode([]byte("If a program is too slow, it must have a loop.\n")),
	}
	m := MapFS{
		"dir": subdir,
	}
	if err := fstest.TestFS(m, "dir", "dir/hello", "dir/fortune", "dir/fortune/k", "dir/fortune/k/ken.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestMapFSChmodDot(t *testing.T) {
	m := MapFS{
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
a: dr-xr-xr-x
a/b.txt: -rw-rw-rw-
`[1:]
	got := buf.String()
	if want != got {
		t.Errorf("MapFS modes want:\n%s\ngot:\n%s\n", want, got)
	}
}

func TestMapFSFileInfoName(t *testing.T) {
	m := MapFS{
		"path/to/b.txt": RawNode(),
	}
	info, _ := fs.Stat(m, "path/to/b.txt")
	want := "b.txt"
	got := info.Name()
	if want != got {
		t.Errorf("MapFS FileInfo.Name want:\n%s\ngot:\n%s\n", want, got)
	}
}

func TestMapFSSub(t *testing.T) {
	subdirFS := MapFS{
		"hello": RawNode([]byte("hello, world\n")),
	}
	m := MapFS{
		"dir": subdirFS,
	}
	sub, err := m.Sub("dir")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sub, subdirFS) {
		t.Fatal("Sub(dir) is not subdirFS")
	}
}
