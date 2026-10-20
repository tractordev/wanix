package bind

import (
	"context"
	"testing"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/memfs"
)

func TestBindNonExistentDirectory(t *testing.T) {
	// Regression: binding a directory that doesn't exist in the source
	// filesystem should auto-create it (mount-point creation), not fail
	// with ErrNotExist. This is the OPFS cold-cache bug: on first load
	// OPFS is empty, so binding #web/opfs/site fails because site/ doesn't
	// exist yet.
	t.Run("AutoMkdir", func(t *testing.T) {
		srcFS := memfs.New() // empty — no "site" directory

		tbl := New()
		ctx := context.Background()

		err := tbl.Bind(ctx, srcFS, "site", "site", fs.BindAfter)
		if err != nil {
			t.Fatalf("Bind non-existent directory: %v", err)
		}

		// The directory should now exist in the source filesystem
		info, err := fs.Stat(srcFS, "site")
		if err != nil {
			t.Fatalf("Stat created directory: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected site to be a directory")
		}
	})

	t.Run("ExistingDirectory", func(t *testing.T) {
		// When the directory already exists, bind should still work (no regression)
		srcFS := memfs.From(fskit.MapFS{
			"site/existing.txt": fskit.RawNode([]byte("hello")),
		})

		tbl := New()
		ctx := context.Background()

		err := tbl.Bind(ctx, srcFS, "site", "site", fs.BindAfter)
		if err != nil {
			t.Fatalf("Bind existing directory: %v", err)
		}

		// Original content should still be accessible
		data, err := fs.ReadFile(srcFS, "site/existing.txt")
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(data) != "hello" {
			t.Errorf("got %q, want %q", string(data), "hello")
		}
	})

	t.Run("NestedNonExistentDirectory", func(t *testing.T) {
		// Binding a/b/c where none of the intermediate directories exist
		srcFS := memfs.New()

		tbl := New()
		ctx := context.Background()

		err := tbl.Bind(ctx, srcFS, "a/b/c", "deep", fs.BindAfter)
		if err != nil {
			t.Fatalf("Bind deeply non-existent directory: %v", err)
		}

		info, err := fs.Stat(srcFS, "a/b/c")
		if err != nil {
			t.Fatalf("Stat created directory: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected a/b/c to be a directory")
		}
	})

	t.Run("BindAfterAutoMkdir", func(t *testing.T) {
		// Verify the bind entry is usable after auto-creation
		srcFS := memfs.New()

		tbl := New()
		ctx := context.Background()

		if err := tbl.Bind(ctx, srcFS, "site", "site", fs.BindAfter); err != nil {
			t.Fatal(err)
		}

		// Check the bind table has the entry
		snap := tbl.Snapshot()
		refs, ok := snap["site"]
		if !ok {
			t.Fatal("expected 'site' in bind table")
		}
		if len(refs) != 1 {
			t.Fatalf("expected 1 ref, got %d", len(refs))
		}
		if refs[0].Path != "site" {
			t.Errorf("ref path = %q, want %q", refs[0].Path, "site")
		}
		if !refs[0].Info.IsDir() {
			t.Error("ref info should report IsDir")
		}
	})
}
