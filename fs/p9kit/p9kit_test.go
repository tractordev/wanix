package p9kit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/hugelgupf/p9/p9"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/memfs"
)

// testSetup creates a connected server and client for testing
// Returns: clientFS, cleanup function
func testSetup(t *testing.T, backend fs.FS) (fs.FS, func()) {
	t.Helper()

	a, b := net.Pipe()
	srv := p9.NewServer(Attacher(backend))

	// Start server in background
	done := make(chan error, 1)
	go func() {
		done <- srv.Handle(a, a)
	}()

	// Create client
	fsys, err := ClientFS(b, "")
	if err != nil {
		t.Fatalf("ClientFS: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		b.Close()
		a.Close()
		// Wait for server to finish
		<-done
	}

	return fsys, cleanup
}

func TestIntegration_BasicReadWrite(t *testing.T) {
	backend := fskit.MapFS{
		"foo1":     fskit.RawNode([]byte("bar1")),
		"foo2":     fskit.RawNode([]byte("bar2")),
		"sub/foo3": fskit.RawNode([]byte("bar3")),
	}

	fsys, cleanup := testSetup(t, backend)
	defer cleanup()

	t.Run("ReadDir root", func(t *testing.T) {
		entries, err := fs.ReadDir(fsys, ".")
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		if len(entries) != 3 {
			t.Fatalf("ReadDir: expected 3 entries, got %d", len(entries))
		}
		if entries[0].Name() != "foo1" {
			t.Fatalf("ReadDir: expected foo1, got %s", entries[0].Name())
		}
	})

	t.Run("ReadFile", func(t *testing.T) {
		data, err := fs.ReadFile(fsys, "foo1")
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(data) != "bar1" {
			t.Errorf("ReadFile: expected 'bar1', got %q", string(data))
		}
	})

	t.Run("Open and Read", func(t *testing.T) {
		f, err := fsys.Open("foo2")
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if string(data) != "bar2" {
			t.Errorf("Read: expected 'bar2', got %q", string(data))
		}
	})

	t.Run("ReadDir subdirectory", func(t *testing.T) {
		entries, err := fs.ReadDir(fsys, "sub")
		if err != nil {
			t.Fatalf("ReadDir(sub): %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("ReadDir: expected 1 entry, got %d", len(entries))
		}
		if entries[0].Name() != "foo3" {
			t.Errorf("ReadDir: expected foo3, got %s", entries[0].Name())
		}
	})
}

func TestIntegration_CreateAndWrite(t *testing.T) {
	backend := memfs.New()
	fsys, cleanup := testSetup(t, backend)
	defer cleanup()

	t.Run("Create new file", func(t *testing.T) {
		f, err := fs.Create(fsys, "new.txt")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		defer f.Close()

		data := []byte("hello world")
		n, err := fs.Write(f, data)
		if err != nil {
			t.Fatalf("Write: %v", err)
		}
		if n != len(data) {
			t.Errorf("Write: expected %d bytes, got %d", len(data), n)
		}
	})

	t.Run("Read created file", func(t *testing.T) {
		data, err := fs.ReadFile(fsys, "new.txt")
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(data) != "hello world" {
			t.Errorf("ReadFile: expected 'hello world', got %q", string(data))
		}
	})

	t.Run("Create existing file truncates", func(t *testing.T) {
		f, err := fs.Create(fsys, "new.txt")
		if err != nil {
			t.Fatalf("Create existing: %v", err)
		}
		defer f.Close()

		data := []byte("new content")
		if _, err := fs.Write(f, data); err != nil {
			t.Fatalf("Write: %v", err)
		}

		// Verify old content is gone
		f.Close()
		newData, err := fs.ReadFile(fsys, "new.txt")
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(newData) != "new content" {
			t.Errorf("Content: expected 'new content', got %q", string(newData))
		}
	})
}

func TestIntegration_FileOperations(t *testing.T) {
	backend := memfs.New()
	fsys, cleanup := testSetup(t, backend)
	defer cleanup()

	t.Run("Seek operations", func(t *testing.T) {
		// Create file with known content
		data := []byte("0123456789")
		if err := fs.WriteFile(backend, "seek.txt", data, 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		f, err := fsys.Open("seek.txt")
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer f.Close()

		// Test SEEK_START
		pos, err := fs.Seek(f, 5, io.SeekStart)
		if err != nil {
			t.Fatalf("Seek(5, SeekStart): %v", err)
		}
		if pos != 5 {
			t.Errorf("Seek: expected pos 5, got %d", pos)
		}

		buf := make([]byte, 1)
		if _, err := f.Read(buf); err != nil {
			t.Fatalf("Read: %v", err)
		}
		if buf[0] != '5' {
			t.Errorf("Read after seek: expected '5', got %c", buf[0])
		}

		// Test SEEK_CURRENT
		pos, err = fs.Seek(f, 2, io.SeekCurrent)
		if err != nil {
			t.Fatalf("Seek(2, SeekCurrent): %v", err)
		}
		if pos != 8 {
			t.Errorf("Seek: expected pos 8, got %d", pos)
		}

		// Test SEEK_END
		pos, err = fs.Seek(f, -3, io.SeekEnd)
		if err != nil {
			t.Fatalf("Seek(-3, SeekEnd): %v", err)
		}
		if pos != 7 {
			t.Errorf("Seek: expected pos 7, got %d", pos)
		}
	})

	t.Run("ReadAt and WriteAt", func(t *testing.T) {
		// Create file with initial content
		initialData := []byte("xxxxxxxxxxxxx") // 13 bytes
		if err := fs.WriteFile(backend, "random.txt", initialData, 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		f, err := fsys.Open("random.txt")
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer f.Close()

		// Write at beginning
		data2 := []byte("hello ")
		n, err := fs.WriteAt(f, data2, 0)
		if err != nil {
			t.Fatalf("WriteAt(0): %v", err)
		}
		if n != len(data2) {
			t.Errorf("WriteAt: expected %d bytes, got %d", len(data2), n)
		}

		// Write at offset
		data := []byte("world")
		n, err = fs.WriteAt(f, data, 6)
		if err != nil {
			t.Fatalf("WriteAt(6): %v", err)
		}
		if n != len(data) {
			t.Errorf("WriteAt: expected %d bytes, got %d", len(data), n)
		}

		// ReadAt to verify
		buf := make([]byte, 11)
		n, err = fs.ReadAt(f, buf, 0)
		if err != nil && err != io.EOF {
			t.Fatalf("ReadAt: %v", err)
		}
		if string(buf) != "hello world" {
			t.Errorf("ReadAt: expected 'hello world', got %q", string(buf))
		}
	})

	t.Run("Stat file", func(t *testing.T) {
		if err := fs.WriteFile(backend, "stat.txt", []byte("test content"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		info, err := fs.Stat(fsys, "stat.txt")
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}

		if info.Name() != "stat.txt" {
			t.Errorf("Name: expected 'stat.txt', got %q", info.Name())
		}
		if info.Size() != 12 {
			t.Errorf("Size: expected 12, got %d", info.Size())
		}
		if info.IsDir() {
			t.Error("IsDir: expected false for file")
		}
	})
}

func TestIntegration_DirectoryOperations(t *testing.T) {
	backend := memfs.New()
	fsys, cleanup := testSetup(t, backend)
	defer cleanup()

	t.Run("Mkdir", func(t *testing.T) {
		if err := fs.Mkdir(fsys, "testdir", 0755); err != nil {
			t.Fatalf("Mkdir: %v", err)
		}

		info, err := fs.Stat(fsys, "testdir")
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if !info.IsDir() {
			t.Error("IsDir: expected true for directory")
		}
	})

	t.Run("Create files in directory", func(t *testing.T) {
		for i, name := range []string{"file1.txt", "file2.txt", "file3.txt"} {
			path := "testdir/" + name
			content := []byte(fmt.Sprintf("content%d", i+1))
			if err := fs.WriteFile(backend, path, content, 0644); err != nil {
				t.Fatalf("WriteFile(%s): %v", path, err)
			}
		}
	})

	t.Run("ReadDir", func(t *testing.T) {
		entries, err := fs.ReadDir(fsys, "testdir")
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		if len(entries) != 3 {
			t.Errorf("ReadDir: expected 3 entries, got %d", len(entries))
		}

		// Verify entries are sorted
		expected := []string{"file1.txt", "file2.txt", "file3.txt"}
		for i, entry := range entries {
			if entry.Name() != expected[i] {
				t.Errorf("Entry[%d]: expected %s, got %s", i, expected[i], entry.Name())
			}
			if entry.IsDir() {
				t.Errorf("Entry[%d]: expected file, got directory", i)
			}
		}
	})

	t.Run("ReadDir on file should fail", func(t *testing.T) {
		if err := fs.WriteFile(backend, "notdir.txt", []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		_, err := fs.ReadDir(fsys, "notdir.txt")
		if err == nil {
			t.Error("ReadDir on file: expected error, got nil")
		}
	})

	t.Run("Nested directories", func(t *testing.T) {
		if err := fs.Mkdir(fsys, "testdir/subdir", 0755); err != nil {
			t.Fatalf("Mkdir nested: %v", err)
		}

		if err := fs.WriteFile(backend, "testdir/subdir/nested.txt", []byte("nested"), 0644); err != nil {
			t.Fatalf("WriteFile nested: %v", err)
		}

		data, err := fs.ReadFile(fsys, "testdir/subdir/nested.txt")
		if err != nil {
			t.Fatalf("ReadFile nested: %v", err)
		}
		if string(data) != "nested" {
			t.Errorf("Content: expected 'nested', got %q", string(data))
		}
	})
}

func TestIntegration_Metadata(t *testing.T) {
	backend := memfs.New()
	fsys, cleanup := testSetup(t, backend)
	defer cleanup()

	// Create a test file
	if err := fs.WriteFile(backend, "meta.txt", []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Run("Chmod", func(t *testing.T) {
		if err := fs.Chmod(fsys, "meta.txt", 0600); err != nil {
			t.Fatalf("Chmod: %v", err)
		}

		info, err := fs.Stat(fsys, "meta.txt")
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}

		perm := info.Mode().Perm()
		if perm != 0600 {
			t.Errorf("Mode: expected 0600, got %o", perm)
		}
	})

	t.Run("Chtimes", func(t *testing.T) {
		newTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		if err := fs.Chtimes(fsys, "meta.txt", newTime, newTime); err != nil {
			t.Fatalf("Chtimes: %v", err)
		}

		info, err := fs.Stat(fsys, "meta.txt")
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}

		if !info.ModTime().Equal(newTime) {
			t.Errorf("ModTime: expected %v, got %v", newTime, info.ModTime())
		}
	})

	t.Run("Truncate grow", func(t *testing.T) {
		if err := fs.Truncate(fsys, "meta.txt", 10); err != nil {
			t.Fatalf("Truncate: %v", err)
		}

		info, err := fs.Stat(fsys, "meta.txt")
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if info.Size() != 10 {
			t.Errorf("Size after truncate: expected 10, got %d", info.Size())
		}
	})

	t.Run("Truncate shrink", func(t *testing.T) {
		if err := fs.Truncate(fsys, "meta.txt", 5); err != nil {
			t.Fatalf("Truncate: %v", err)
		}

		info, err := fs.Stat(fsys, "meta.txt")
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if info.Size() != 5 {
			t.Errorf("Size after truncate: expected 5, got %d", info.Size())
		}
	})
}

func TestIntegration_RenameAndRemove(t *testing.T) {
	backend := memfs.New()
	fsys, cleanup := testSetup(t, backend)
	defer cleanup()

	t.Run("Rename file", func(t *testing.T) {
		if err := fs.WriteFile(backend, "old.txt", []byte("content"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		if err := fs.Rename(fsys, "old.txt", "new.txt"); err != nil {
			t.Fatalf("Rename: %v", err)
		}

		// Old name should not exist
		_, err := fs.Stat(fsys, "old.txt")
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Stat(old): expected not exist, got %v", err)
		}

		// New name should exist with same content
		data, err := fs.ReadFile(fsys, "new.txt")
		if err != nil {
			t.Fatalf("ReadFile(new): %v", err)
		}
		if string(data) != "content" {
			t.Errorf("Content: expected 'content', got %q", string(data))
		}
	})

	t.Run("Rename to different directory", func(t *testing.T) {
		if err := fs.Mkdir(fsys, "dir1", 0755); err != nil {
			t.Fatalf("Mkdir: %v", err)
		}
		if err := fs.Mkdir(fsys, "dir2", 0755); err != nil {
			t.Fatalf("Mkdir: %v", err)
		}
		if err := fs.WriteFile(backend, "dir1/file.txt", []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		if err := fs.Rename(fsys, "dir1/file.txt", "dir2/file.txt"); err != nil {
			t.Fatalf("Rename across dirs: %v", err)
		}

		// Verify move
		_, err := fs.Stat(fsys, "dir1/file.txt")
		if !errors.Is(err, fs.ErrNotExist) {
			t.Error("Old path should not exist")
		}

		data, err := fs.ReadFile(fsys, "dir2/file.txt")
		if err != nil {
			t.Fatalf("ReadFile(new): %v", err)
		}
		if string(data) != "test" {
			t.Errorf("Content: expected 'test', got %q", string(data))
		}
	})

	t.Run("Remove file", func(t *testing.T) {
		if err := fs.WriteFile(backend, "remove.txt", []byte("delete me"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		if err := fs.Remove(fsys, "remove.txt"); err != nil {
			t.Fatalf("Remove: %v", err)
		}

		_, err := fs.Stat(fsys, "remove.txt")
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Stat after remove: expected not exist, got %v", err)
		}
	})

	t.Run("Remove empty directory", func(t *testing.T) {
		if err := fs.Mkdir(fsys, "emptydir", 0755); err != nil {
			t.Fatalf("Mkdir: %v", err)
		}

		if err := fs.Remove(fsys, "emptydir"); err != nil {
			t.Fatalf("Remove dir: %v", err)
		}

		_, err := fs.Stat(fsys, "emptydir")
		if !errors.Is(err, fs.ErrNotExist) {
			t.Error("Directory should not exist after remove")
		}
	})
}

func TestIntegration_Symlinks(t *testing.T) {
	backend := memfs.New()
	fsys, cleanup := testSetup(t, backend)
	defer cleanup()

	t.Run("Create and read symlink", func(t *testing.T) {
		// Create target file
		if err := fs.WriteFile(backend, "target.txt", []byte("target content"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		// Create symlink
		if err := fs.Symlink(fsys, "target.txt", "link.txt"); err != nil {
			t.Fatalf("Symlink: %v", err)
		}

		// Read symlink
		target, err := fs.Readlink(fsys, "link.txt")
		if err != nil {
			t.Fatalf("Readlink: %v", err)
		}
		if target != "target.txt" {
			t.Errorf("Readlink: expected 'target.txt', got %q", target)
		}
	})

	t.Run("Stat symlink with NoFollow", func(t *testing.T) {
		info, err := fs.StatContext(fs.WithNoFollow(context.Background()), fsys, "link.txt")
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			t.Error("Mode: expected symlink flag to be set")
		}
	})

	t.Run("Symlink to nonexistent target", func(t *testing.T) {
		// Symlinks can point to nonexistent files
		if err := fs.Symlink(fsys, "nonexistent.txt", "broken.txt"); err != nil {
			t.Fatalf("Symlink to nonexistent: %v", err)
		}

		target, err := fs.Readlink(fsys, "broken.txt")
		if err != nil {
			t.Fatalf("Readlink: %v", err)
		}
		if target != "nonexistent.txt" {
			t.Errorf("Readlink: expected 'nonexistent.txt', got %q", target)
		}
	})
}

func TestIntegration_ErrorHandling(t *testing.T) {
	backend := memfs.New()
	fsys, cleanup := testSetup(t, backend)
	defer cleanup()

	t.Run("Open nonexistent file", func(t *testing.T) {
		_, err := fsys.Open("nonexistent.txt")
		if err == nil {
			t.Fatal("Open: expected error, got nil")
		}
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Open: expected ErrNotExist, got %v", err)
		}
	})

	t.Run("Stat nonexistent file", func(t *testing.T) {
		_, err := fs.Stat(fsys, "nonexistent.txt")
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Stat: expected ErrNotExist, got %v", err)
		}
	})

	t.Run("Mkdir existing", func(t *testing.T) {
		if err := fs.Mkdir(fsys, "existing", 0755); err != nil {
			t.Fatalf("Mkdir: %v", err)
		}

		err := fs.Mkdir(fsys, "existing", 0755)
		if err == nil {
			t.Fatal("Mkdir existing: expected error, got nil")
		}
	})

	t.Run("Invalid paths", func(t *testing.T) {
		invalidPaths := []string{"", "../etc/passwd", "foo/../../../bar"}
		for _, path := range invalidPaths {
			_, err := fsys.Open(path)
			if err == nil {
				t.Errorf("Open(%q): expected error, got nil", path)
			}
		}
	})

	t.Run("Remove nonexistent", func(t *testing.T) {
		err := fs.Remove(fsys, "nonexistent.txt")
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Remove: expected ErrNotExist, got %v", err)
		}
	})

	t.Run("Remove root directory", func(t *testing.T) {
		err := fs.Remove(fsys, ".")
		if err == nil {
			t.Error("Remove(.): expected error, got nil")
		}
	})

	t.Run("Readlink on regular file", func(t *testing.T) {
		if err := fs.WriteFile(backend, "regular.txt", []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		_, err := fs.Readlink(fsys, "regular.txt")
		if err == nil {
			t.Error("Readlink on regular file: expected error, got nil")
		}
	})
}

func TestIntegration_ContextSupport(t *testing.T) {
	backend := memfs.New()
	fsys, cleanup := testSetup(t, backend)
	defer cleanup()

	// Create test file
	if err := fs.WriteFile(backend, "test.txt", []byte("test content"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Run("OpenContext", func(t *testing.T) {
		ctx := context.Background()
		f, err := fs.OpenContext(ctx, fsys, "test.txt")
		if err != nil {
			t.Fatalf("OpenContext: %v", err)
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if string(data) != "test content" {
			t.Errorf("Read: expected 'test content', got %q", string(data))
		}
	})

	t.Run("StatContext", func(t *testing.T) {
		ctx := context.Background()
		info, err := fs.StatContext(ctx, fsys, "test.txt")
		if err != nil {
			t.Fatalf("StatContext: %v", err)
		}
		if info.Name() != "test.txt" {
			t.Errorf("Name: expected 'test.txt', got %q", info.Name())
		}
		if info.Size() != 12 {
			t.Errorf("Size: expected 12, got %d", info.Size())
		}
	})
}

func TestIntegration_ConcurrentAccess(t *testing.T) {
	backend := memfs.New()
	fsys, cleanup := testSetup(t, backend)
	defer cleanup()

	t.Run("Concurrent creates", func(t *testing.T) {
		const n = 10
		done := make(chan error, n)

		for i := 0; i < n; i++ {
			i := i
			go func() {
				name := fmt.Sprintf("file%d.txt", i)
				f, err := fs.Create(fsys, name)
				if err != nil {
					done <- err
					return
				}
				content := fmt.Sprintf("content%d", i)
				_, err = fs.Write(f, []byte(content))
				f.Close()
				done <- err
			}()
		}

		for i := 0; i < n; i++ {
			if err := <-done; err != nil {
				t.Errorf("Concurrent create %d: %v", i, err)
			}
		}

		// Verify all files exist
		entries, err := fs.ReadDir(fsys, ".")
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		if len(entries) != n {
			t.Errorf("ReadDir: expected %d entries, got %d", n, len(entries))
		}
	})

	t.Run("Concurrent reads", func(t *testing.T) {
		// Create a file to read
		if err := fs.WriteFile(backend, "shared.txt", []byte("shared content"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		const n = 20
		done := make(chan error, n)

		for i := 0; i < n; i++ {
			go func() {
				data, err := fs.ReadFile(fsys, "shared.txt")
				if err != nil {
					done <- err
					return
				}
				if string(data) != "shared content" {
					done <- fmt.Errorf("unexpected content: %q", string(data))
					return
				}
				done <- nil
			}()
		}

		for i := 0; i < n; i++ {
			if err := <-done; err != nil {
				t.Errorf("Concurrent read %d: %v", i, err)
			}
		}
	})
}
