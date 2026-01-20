package fs

import (
	"context"
	"io/fs"
	"testing"
	"time"
)

// testFS is a filesystem that implements many interfaces.
// It tracks which methods were called.
type testFS struct {
	calls []string
}

func (t *testFS) Open(name string) (fs.File, error) {
	t.calls = append(t.calls, "Open")
	return nil, ErrNotExist
}

func (t *testFS) Mkdir(name string, perm FileMode) error {
	t.calls = append(t.calls, "Mkdir")
	return nil
}

func (t *testFS) Remove(name string) error {
	t.calls = append(t.calls, "Remove")
	return nil
}

func (t *testFS) Rename(oldname, newname string) error {
	t.calls = append(t.calls, "Rename")
	return nil
}

func (t *testFS) Chmod(name string, mode FileMode) error {
	t.calls = append(t.calls, "Chmod")
	return nil
}

func (t *testFS) Chown(name string, uid, gid int) error {
	t.calls = append(t.calls, "Chown")
	return nil
}

func (t *testFS) Chtimes(name string, atime, mtime time.Time) error {
	t.calls = append(t.calls, "Chtimes")
	return nil
}

func (t *testFS) Create(name string) (File, error) {
	t.calls = append(t.calls, "Create")
	return nil, nil
}

func (t *testFS) Symlink(oldname, newname string) error {
	t.calls = append(t.calls, "Symlink")
	return nil
}

func (t *testFS) Readlink(name string) (string, error) {
	t.calls = append(t.calls, "Readlink")
	return "", nil
}

func (t *testFS) Truncate(name string, size int64) error {
	t.calls = append(t.calls, "Truncate")
	return nil
}

func (t *testFS) OpenFile(name string, flag int, perm FileMode) (File, error) {
	t.calls = append(t.calls, "OpenFile")
	return nil, nil
}

func (t *testFS) SetXattr(ctx context.Context, name, attr string, data []byte, flags int) error {
	t.calls = append(t.calls, "SetXattr")
	return nil
}

func (t *testFS) GetXattr(ctx context.Context, name, attr string) ([]byte, error) {
	t.calls = append(t.calls, "GetXattr")
	return nil, nil
}

func (t *testFS) ListXattrs(ctx context.Context, name string) ([]string, error) {
	t.calls = append(t.calls, "ListXattrs")
	return nil, nil
}

func (t *testFS) RemoveXattr(ctx context.Context, name, attr string) error {
	t.calls = append(t.calls, "RemoveXattr")
	return nil
}

// Verify testFS implements all these interfaces
var (
	_ MkdirFS    = (*testFS)(nil)
	_ RemoveFS   = (*testFS)(nil)
	_ RenameFS   = (*testFS)(nil)
	_ ChmodFS    = (*testFS)(nil)
	_ ChownFS    = (*testFS)(nil)
	_ ChtimesFS  = (*testFS)(nil)
	_ CreateFS   = (*testFS)(nil)
	_ SymlinkFS  = (*testFS)(nil)
	_ ReadlinkFS = (*testFS)(nil)
	_ TruncateFS = (*testFS)(nil)
	_ OpenFileFS = (*testFS)(nil)
	_ XattrFS    = (*testFS)(nil)
)

// wrapperFS embeds DefaultFS and only overrides Open.
// Without DefaultFS, type assertions for the embedded testFS interfaces would fail.
type wrapperFS struct {
	*DefaultFS
	openCalled bool
}

func (w *wrapperFS) Open(name string) (fs.File, error) {
	w.openCalled = true
	return w.DefaultFS.FS.Open(name)
}

func TestDefaultFS_InterfacePassthrough(t *testing.T) {
	// Create a testFS that implements many interfaces
	original := &testFS{}

	// Wrap with DefaultFS and embed in a new struct
	wrapped := &wrapperFS{
		DefaultFS: NewDefault(original),
	}

	// Now use the fs package functions on the wrapper.
	// These should find the original testFS implementations via DefaultFS methods.

	t.Run("Mkdir", func(t *testing.T) {
		original.calls = nil
		err := Mkdir(wrapped, "test", 0755)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "Mkdir" {
			t.Errorf("expected Mkdir to be called on original, got: %v", original.calls)
		}
	})

	t.Run("Remove", func(t *testing.T) {
		original.calls = nil
		err := Remove(wrapped, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "Remove" {
			t.Errorf("expected Remove to be called on original, got: %v", original.calls)
		}
	})

	t.Run("Rename", func(t *testing.T) {
		original.calls = nil
		err := Rename(wrapped, "old", "new")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "Rename" {
			t.Errorf("expected Rename to be called on original, got: %v", original.calls)
		}
	})

	t.Run("Chmod", func(t *testing.T) {
		original.calls = nil
		err := Chmod(wrapped, "test", 0644)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "Chmod" {
			t.Errorf("expected Chmod to be called on original, got: %v", original.calls)
		}
	})

	t.Run("Chown", func(t *testing.T) {
		original.calls = nil
		err := Chown(wrapped, "test", 1000, 1000)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "Chown" {
			t.Errorf("expected Chown to be called on original, got: %v", original.calls)
		}
	})

	t.Run("Chtimes", func(t *testing.T) {
		original.calls = nil
		err := Chtimes(wrapped, "test", time.Now(), time.Now())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "Chtimes" {
			t.Errorf("expected Chtimes to be called on original, got: %v", original.calls)
		}
	})

	t.Run("Create", func(t *testing.T) {
		original.calls = nil
		_, err := Create(wrapped, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "Create" {
			t.Errorf("expected Create to be called on original, got: %v", original.calls)
		}
	})

	t.Run("Symlink", func(t *testing.T) {
		original.calls = nil
		err := Symlink(wrapped, "target", "link")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "Symlink" {
			t.Errorf("expected Symlink to be called on original, got: %v", original.calls)
		}
	})

	t.Run("Readlink", func(t *testing.T) {
		original.calls = nil
		_, err := Readlink(wrapped, "link")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "Readlink" {
			t.Errorf("expected Readlink to be called on original, got: %v", original.calls)
		}
	})

	t.Run("Truncate", func(t *testing.T) {
		original.calls = nil
		err := Truncate(wrapped, "test", 100)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "Truncate" {
			t.Errorf("expected Truncate to be called on original, got: %v", original.calls)
		}
	})

	t.Run("OpenFile", func(t *testing.T) {
		original.calls = nil
		_, err := OpenFile(wrapped, "test", 0, 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "OpenFile" {
			t.Errorf("expected OpenFile to be called on original, got: %v", original.calls)
		}
	})

	t.Run("SetXattr", func(t *testing.T) {
		original.calls = nil
		err := SetXattr(context.Background(), wrapped, "test", "attr", []byte("data"), 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "SetXattr" {
			t.Errorf("expected SetXattr to be called on original, got: %v", original.calls)
		}
	})

	t.Run("GetXattr", func(t *testing.T) {
		original.calls = nil
		_, err := GetXattr(context.Background(), wrapped, "test", "attr")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "GetXattr" {
			t.Errorf("expected GetXattr to be called on original, got: %v", original.calls)
		}
	})

	t.Run("ListXattrs", func(t *testing.T) {
		original.calls = nil
		_, err := ListXattrs(context.Background(), wrapped, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "ListXattrs" {
			t.Errorf("expected ListXattrs to be called on original, got: %v", original.calls)
		}
	})

	t.Run("RemoveXattr", func(t *testing.T) {
		original.calls = nil
		err := RemoveXattr(context.Background(), wrapped, "test", "attr")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(original.calls) != 1 || original.calls[0] != "RemoveXattr" {
			t.Errorf("expected RemoveXattr to be called on original, got: %v", original.calls)
		}
	})
}

// shadowingWrapperFS embeds fs.FS (interface) instead of a concrete type.
// This is a common pattern when you want to wrap any FS, not just a specific one.
// Without DefaultFS, the wrapper won't implement any extended interfaces.
type shadowingWrapperFS struct {
	fs.FS
	openCalled bool
}

func (w *shadowingWrapperFS) Open(name string) (fs.File, error) {
	w.openCalled = true
	return w.FS.Open(name)
}

// TestInterfaceEmbed_Problem demonstrates that when you embed fs.FS (the interface)
// rather than a concrete type, you lose access to all the extended interfaces.
// This is the problem DefaultFS solves.
func TestInterfaceEmbed_Problem(t *testing.T) {
	original := &testFS{}

	// Embed fs.FS interface directly - common pattern for generic wrappers
	direct := &shadowingWrapperFS{FS: original}

	// Type assertions fail because direct only embeds fs.FS interface,
	// even though original implements MkdirFS
	if _, ok := (fs.FS)(direct).(MkdirFS); ok {
		t.Error("Interface embedding: MkdirFS assertion should fail")
	} else {
		t.Log("Interface embedding: MkdirFS assertion failed (this is the problem)")
	}

	// Now wrap with DefaultFS - the wrapper gains all the interface implementations
	wrapped := &wrapperFS{DefaultFS: NewDefault(original)}

	if _, ok := (fs.FS)(wrapped).(MkdirFS); !ok {
		t.Error("DefaultFS wrapper: MkdirFS assertion should succeed")
	} else {
		t.Log("DefaultFS wrapper: MkdirFS assertion succeeded (DefaultFS solved the problem)")
	}

	// Verify that operations actually reach the original FS
	original.calls = nil
	Mkdir(wrapped, "test", 0755)
	if len(original.calls) != 1 || original.calls[0] != "Mkdir" {
		t.Errorf("expected Mkdir to reach original, got: %v", original.calls)
	}
}
