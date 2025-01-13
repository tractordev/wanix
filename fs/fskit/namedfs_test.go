package fskit

import (
	"testing"

	"tractor.dev/wanix/fs"
)

func TestNamedFile(t *testing.T) {
	f, _ := Entry("foo", 0777, []byte("hello, world\n")).Open(".")
	nf := renamedFile{File: f, name: "bar"}
	fi, _ := nf.Stat()
	if fi.Name() != "bar" {
		t.Fatal("name not set")
	}
}

func TestNamedFS(t *testing.T) {
	fsys := MapFS{
		"foo": Entry("foo", 0777),
	}

	fi, err := fs.Stat(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if fi.Name() != "." {
		t.Fatal("name not set to . as expected by default")
	}

	renamedFsys := NamedFS(fsys, "bar")
	fi, err = fs.Stat(renamedFsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if fi.Name() != "bar" {
		t.Fatal("name not set to bar as expected")
	}
}
