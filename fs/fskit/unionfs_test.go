package fskit

import (
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"
)

func TestUnionFS(t *testing.T) {
	fsys1 := fstest.MapFS{
		"foo":          &fstest.MapFile{Data: []byte("hello")},
		"fsys1/foo":    &fstest.MapFile{Data: []byte("hello")},
		"common/file1": &fstest.MapFile{Data: []byte("hello")},
	}
	fsys2 := fstest.MapFS{
		"bar":          &fstest.MapFile{Data: []byte("world")},
		"fsys2/foo":    &fstest.MapFile{Data: []byte("world")},
		"common/file2": &fstest.MapFile{Data: []byte("world")},
	}
	union := UnionFS{fsys1, fsys2}
	if err := fstest.TestFS(union,
		"foo",
		"bar",
		"fsys1/foo",
		"fsys2/foo",
		"common/file1",
		"common/file2",
	); err != nil {
		t.Fatalf("TestFS failed: %v", err)
	}
}

func TestUnionFS_ImplicitDirs(t *testing.T) {
	fsys1 := fstest.MapFS{
		"a/b/file1": &fstest.MapFile{Data: []byte("hello")},
	}
	fsys2 := fstest.MapFS{
		"a/c/file2": &fstest.MapFile{Data: []byte("world")},
	}

	union := UnionFS{fsys1, fsys2}

	// Test that we can open the implicit "a" directory
	dir, err := union.Open("a")
	if err != nil {
		t.Fatalf("failed to open implicit dir: %v", err)
	}
	defer dir.Close()

	// Test that we can list the contents
	if dirFile, ok := dir.(fs.ReadDirFile); ok {
		entries, err := dirFile.ReadDir(-1)
		if err != nil {
			t.Fatalf("failed to read dir: %v", err)
		}

		// Should see both "b" and "c" directories
		names := make([]string, len(entries))
		for i, entry := range entries {
			names[i] = entry.Name()
		}
		expected := []string{"b", "c"}
		if !slices.Equal(names, expected) {
			t.Errorf("got entries %v, want %v", names, expected)
		}
	}
}
