package memfs

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

func TestMemFSCreate(t *testing.T) {
	m := From(fskit.MapFS{
		"hello": fskit.RawNode([]byte("hello, world\n")),
	})

	// check for failure if parent directory does not exist
	if _, err := fs.Create(m, "foo/bar"); err == nil {
		t.Fatal("expected error that parent directory does not exist")
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
	m := From(fskit.MapFS{
		"hello": fskit.RawNode([]byte("hello, world\n")),
	})

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
	m := From(fskit.MapFS{
		"hello": fskit.RawNode([]byte("hello, world\n")),
	})

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
	m := From(fskit.MapFS{
		"hello": fskit.RawNode([]byte("hello, world\n"), fs.FileMode(0666)),
	})

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
	m := From(fskit.MapFS{
		"hello":   fskit.RawNode([]byte("hello, world\n")),
		"foo/bar": fskit.RawNode([]byte("foobar\n")),
	})

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
	t.Run("FileRename", func(t *testing.T) {
		m := From(fskit.MapFS{
			"hello": fskit.RawNode([]byte("hello, world\n")),
		})

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
	})

	t.Run("DirectoryRename", func(t *testing.T) {
		m := From(fskit.MapFS{
			"dir/a.txt":     fskit.RawNode([]byte("file a\n")),
			"dir/b.txt":     fskit.RawNode([]byte("file b\n")),
			"dir/sub/c.txt": fskit.RawNode([]byte("file c\n")),
		})

		// Rename directory with children
		if err := fs.Rename(m, "dir", "newdir"); err != nil {
			t.Fatalf("rename directory: %v", err)
		}

		// Verify old paths are gone
		if _, err := fs.Stat(m, "dir"); err == nil {
			t.Error("old directory path should not exist")
		}
		if _, err := fs.Stat(m, "dir/a.txt"); err == nil {
			t.Error("old child path should not exist")
		}
		if _, err := fs.Stat(m, "dir/sub/c.txt"); err == nil {
			t.Error("old descendant path should not exist")
		}

		// Verify new paths exist with correct content
		if _, err := fs.Stat(m, "newdir"); err != nil {
			t.Errorf("new directory path should exist: %v", err)
		}

		data, err := fs.ReadFile(m, "newdir/a.txt")
		if err != nil {
			t.Errorf("new child path should exist: %v", err)
		} else if string(data) != "file a\n" {
			t.Errorf("file content mismatch: got %q, want %q", string(data), "file a\n")
		}

		data, err = fs.ReadFile(m, "newdir/sub/c.txt")
		if err != nil {
			t.Errorf("new descendant path should exist: %v", err)
		} else if string(data) != "file c\n" {
			t.Errorf("file content mismatch: got %q, want %q", string(data), "file c\n")
		}

		// Test directory listing
		entries, err := fs.ReadDir(m, "newdir")
		if err != nil {
			t.Fatalf("readdir newdir: %v", err)
		}
		if len(entries) != 3 { // a.txt, b.txt, sub
			t.Errorf("expected 3 entries in newdir, got %d", len(entries))
		}
	})

	t.Run("OverwriteFile", func(t *testing.T) {
		m := From(fskit.MapFS{
			"old.txt": fskit.RawNode([]byte("old content\n")),
			"new.txt": fskit.RawNode([]byte("new content\n")),
		})

		// Rename should overwrite existing file
		if err := fs.Rename(m, "old.txt", "new.txt"); err != nil {
			t.Fatalf("rename over file: %v", err)
		}

		// Verify old file is gone
		if _, err := fs.Stat(m, "old.txt"); err == nil {
			t.Error("old file should not exist")
		}

		// Verify new file has old content
		data, err := fs.ReadFile(m, "new.txt")
		if err != nil {
			t.Fatalf("read new.txt: %v", err)
		}
		if string(data) != "old content\n" {
			t.Errorf("file content = %q, want %q", string(data), "old content\n")
		}
	})

	t.Run("OverwriteEmptyDirectory", func(t *testing.T) {
		m := From(fskit.MapFS{
			"old.txt": fskit.RawNode([]byte("content\n")),
		})
		if err := fs.Mkdir(m, "newdir", 0755); err != nil {
			t.Fatal(err)
		}

		// Rename file over empty directory should succeed
		if err := fs.Rename(m, "old.txt", "newdir"); err != nil {
			t.Fatalf("rename over empty dir: %v", err)
		}

		// Verify it's now a file
		info, err := fs.Stat(m, "newdir")
		if err != nil {
			t.Fatal(err)
		}
		if info.IsDir() {
			t.Error("newdir should be a file now")
		}
	})

	t.Run("OverwriteNonEmptyDirectory", func(t *testing.T) {
		m := From(fskit.MapFS{
			"old.txt":      fskit.RawNode([]byte("content\n")),
			"newdir/a.txt": fskit.RawNode([]byte("child\n")),
		})

		// Rename over non-empty directory should fail
		if err := fs.Rename(m, "old.txt", "newdir"); err == nil {
			t.Error("rename over non-empty directory should fail")
		}
	})
}

func TestMemFSSymlink(t *testing.T) {
	m := New()

	// Test symlink with non-existent parent directory
	if err := fs.Symlink(m, "target.txt", "foo/link.txt"); err == nil {
		t.Error("symlink with non-existent parent should fail")
	}

	// Test symlink with existing parent
	if err := fs.Mkdir(m, "dir", 0755); err != nil {
		t.Fatal(err)
	}
	if err := fs.Symlink(m, "target.txt", "dir/link.txt"); err != nil {
		t.Fatalf("symlink with existing parent: %v", err)
	}

	// Verify symlink exists and target is correct
	target, err := fs.Readlink(m, "dir/link.txt")
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != "target.txt" {
		t.Errorf("readlink = %q, want %q", target, "target.txt")
	}
}

func TestMemFSReadlink(t *testing.T) {
	m := New()

	// Test readlink on non-existent file
	if _, err := fs.Readlink(m, "nonexistent"); err == nil {
		t.Error("readlink on non-existent file should fail")
	}

	// Test readlink on regular file
	if err := fs.WriteFile(m, "file.txt", []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Readlink(m, "file.txt"); err == nil {
		t.Error("readlink on regular file should fail")
	}

	// Test readlink on symlink
	if err := fs.Symlink(m, "target.txt", "link.txt"); err != nil {
		t.Fatal(err)
	}
	target, err := fs.Readlink(m, "link.txt")
	if err != nil {
		t.Fatalf("readlink on symlink: %v", err)
	}
	if target != "target.txt" {
		t.Errorf("readlink = %q, want %q", target, "target.txt")
	}
}

func TestMemFS(t *testing.T) {
	m := From(fskit.MapFS{
		"hello":             fskit.RawNode([]byte("hello, world\n")),
		"fortune/k/ken.txt": fskit.RawNode([]byte("If a program is too slow, it must have a loop.\n")),
	})
	if err := fstest.TestFS(m, "hello", "fortune", "fortune/k", "fortune/k/ken.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestMemFSChmodDot(t *testing.T) {
	m := From(fskit.MapFS{
		"a/b.txt": fskit.RawNode(fs.FileMode(0666)),
		".":       fskit.RawNode(fs.FileMode(0777 | fs.ModeDir)),
	})
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
		t.Errorf("fskit.MapFS modes want:\n%s\ngot:\n%s\n", want, got)
	}
}

func TestMemFSFileInfoName(t *testing.T) {
	m := From(fskit.MapFS{
		"path/to/b.txt": fskit.RawNode(),
	})
	info, _ := fs.Stat(m, "path/to/b.txt")
	want := "b.txt"
	got := info.Name()
	if want != got {
		t.Errorf("fskit.MapFS FileInfo.Name want:\n%s\ngot:\n%s\n", want, got)
	}
}

func TestMemFSSymlinks(t *testing.T) {
	m := From(fskit.MapFS{
		"path/to/b.txt": fskit.RawNode([]byte("contents")),
		"file":          fskit.RawNode([]byte("path/to/b.txt"), fs.ModeSymlink),
		"dir":           fskit.RawNode([]byte("path/to"), fs.ModeSymlink),
	})

	t.Run("Readlink returns target of symlink", func(t *testing.T) {
		target, err := fs.Readlink(m, "file")
		if err != nil {
			t.Fatal(err)
		}
		want := []byte("path/to/b.txt")
		if !bytes.Equal(want, []byte(target)) {
			t.Errorf("Readlink want:\n%s\ngot:\n%s\n", want, target)
		}
	})

	t.Run("ReadFile follows symlinks", func(t *testing.T) {
		b, err := fs.ReadFile(m, "file")
		if err != nil {
			t.Fatal(err)
		}
		want := []byte("contents")
		if !bytes.Equal(want, b) {
			t.Errorf("ReadFile want:\n%s\ngot:\n%s\n", want, b)
		}
	})

	t.Run("Stat follows file symlink", func(t *testing.T) {
		info, err := fs.Stat(m, "file")
		if err != nil {
			t.Fatal(err)
		}
		want := "b.txt"
		got := info.Name()
		if want != got {
			t.Errorf("FileInfo.Name want: %q, got: %q", want, got)
		}
	})

	t.Run("Stat follows directory symlink", func(t *testing.T) {
		info, err := fs.Stat(m, "dir")
		if err != nil {
			t.Fatal(err)
		}
		want := "to"
		got := info.Name()
		if want != got {
			t.Errorf("FileInfo.Name want: %q, got: %q", want, got)
		}
	})

	t.Run("StatContext with NoFollow returns symlink info", func(t *testing.T) {
		info, err := fs.StatContext(fs.WithNoFollow(context.Background()), m, "dir")
		if err != nil {
			t.Fatal(err)
		}
		want := "dir"
		got := info.Name()
		if want != got {
			t.Errorf("FileInfo.Name want: %q, got: %q", want, got)
		}
	})

	t.Run("Symlink makes a symlink and symlinks can target symlinks", func(t *testing.T) {
		err := fs.Symlink(m, "file", "symlink")
		if err != nil {
			t.Fatal(err)
		}

		b, err := fs.ReadFile(m, "symlink")
		if err != nil {
			t.Fatal(err)
		}
		want := []byte("contents")
		if !bytes.Equal(want, b) {
			t.Errorf("ReadFile want:\n%s\ngot:\n%s\n", want, b)
		}
	})
}

func TestTruncate(t *testing.T) {
	t.Run("TruncateBasic", func(t *testing.T) {
		m := New()

		// Create file with content
		if err := fs.WriteFile(m, "file.txt", []byte("hello world"), 0644); err != nil {
			t.Fatal(err)
		}

		// Truncate to smaller size
		if err := fs.Truncate(m, "file.txt", 5); err != nil {
			t.Fatal(err)
		}

		data, err := fs.ReadFile(m, "file.txt")
		if err != nil {
			t.Fatal(err)
		}

		if string(data) != "hello" {
			t.Errorf("expected 'hello', got '%s'", string(data))
		}
	})

	t.Run("TruncateExtend", func(t *testing.T) {
		m := New()

		// Create file with content
		if err := fs.WriteFile(m, "file.txt", []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}

		// Truncate to larger size (should pad with null bytes)
		if err := fs.Truncate(m, "file.txt", 10); err != nil {
			t.Fatal(err)
		}

		data, err := fs.ReadFile(m, "file.txt")
		if err != nil {
			t.Fatal(err)
		}

		if len(data) != 10 {
			t.Errorf("expected length 10, got %d", len(data))
		}

		expected := []byte{'h', 'e', 'l', 'l', 'o', 0, 0, 0, 0, 0}
		if !bytes.Equal(data, expected) {
			t.Errorf("expected %v, got %v", expected, data)
		}
	})

	t.Run("TruncateWithOpenHandle", func(t *testing.T) {
		// This is the key test for the vi bug fix
		m := New()

		// Create file with initial content
		if err := fs.WriteFile(m, "test.txt", []byte("hello world"), 0644); err != nil {
			t.Fatal(err)
		}

		// Open file for writing (simulating vi opening the file)
		f, err := m.Open("test.txt")
		if err != nil {
			t.Fatal(err)
		}

		// Truncate the file while it's open (simulating vi's SetAttr truncate)
		if err := fs.Truncate(m, "test.txt", 0); err != nil {
			t.Fatal(err)
		}

		// Write to the open file handle (simulating vi writing)
		if w, ok := f.(interface{ Write([]byte) (int, error) }); ok {
			n, err := w.Write([]byte("test content"))
			if err != nil {
				t.Fatal(err)
			}
			if n != 12 {
				t.Errorf("expected to write 12 bytes, wrote %d", n)
			}
		} else {
			t.Fatal("file doesn't support Write")
		}

		// Close the file handle
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}

		// Read the file - should contain "test content", not be empty or corrupted
		data, err := fs.ReadFile(m, "test.txt")
		if err != nil {
			t.Fatal(err)
		}

		if string(data) != "test content" {
			t.Errorf("expected 'test content', got '%s' (bytes: %v)", string(data), data)
		}
	})

	t.Run("TruncateMultipleHandles", func(t *testing.T) {
		// Test with multiple file handles open simultaneously
		// This demonstrates that each handle has its own buffer snapshot
		m := New()

		// Create file
		if err := fs.WriteFile(m, "test.txt", []byte("original content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Open first handle
		f1, err := m.Open("test.txt")
		if err != nil {
			t.Fatal(err)
		}

		// Open second handle
		f2, err := m.Open("test.txt")
		if err != nil {
			t.Fatal(err)
		}

		// Truncate the underlying node
		if err := fs.Truncate(m, "test.txt", 0); err != nil {
			t.Fatal(err)
		}

		// Write from first handle - writes to its buffer which had "original content"
		if w, ok := f1.(interface{ Write([]byte) (int, error) }); ok {
			w.Write([]byte("from f1"))
		}

		// Close first handle - syncs its buffer to node
		f1.Close()

		// At this point the file contains "from f1l content" because f1's buffer
		// had "original content" and we overwrote the first 7 bytes

		// Write from second handle - writes to its buffer
		if w, ok := f2.(interface{ Write([]byte) (int, error) }); ok {
			w.Write([]byte("from f2"))
		}

		// Close second handle - syncs its buffer to node
		f2.Close()

		// The second write overwrites because it closed last
		data, err := fs.ReadFile(m, "test.txt")
		if err != nil {
			t.Fatal(err)
		}

		// Both handles had copies of "original content" in their buffers
		// Both overwrote the first 7 bytes with their write
		// The last close wins, so we get f2's modified buffer
		expected := "from f2l content"
		if string(data) != expected {
			t.Errorf("expected '%s', got '%s'", expected, string(data))
		}
	})
}
