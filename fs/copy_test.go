package fs_test

import (
	"os"
	"testing"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/memfs"
)

func TestCopyAll_File(t *testing.T) {
	// Create filesystem with a test file
	fsys := memfs.New()
	content := []byte("Hello, World!")
	fsys.SetNode("test.txt", fskit.Entry("test.txt", 0644, content, time.Now()))

	// Test copying a file within the same filesystem
	err := fs.CopyAll(fsys, "test.txt", "copy.txt")
	if err != nil {
		t.Fatalf("CopyAll failed: %v", err)
	}

	// Verify the file was copied
	dstFile, err := fsys.Open("copy.txt")
	if err != nil {
		t.Fatalf("Failed to open copied file: %v", err)
	}
	defer dstFile.Close()

	// Read and verify content
	buf := make([]byte, len(content))
	n, err := dstFile.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if n != len(content) || string(buf) != string(content) {
		t.Errorf("Content mismatch: got %q, want %q", string(buf), string(content))
	}

	// Verify file info
	info, err := dstFile.Stat()
	if err != nil {
		t.Fatalf("Failed to stat copied file: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("Permission mismatch: got %o, want %o", info.Mode().Perm(), 0644)
	}
}

func TestCopyAll_Directory(t *testing.T) {
	// Create filesystem with a directory structure
	fsys := memfs.New()
	now := time.Now()

	// Create directory structure
	fsys.SetNode("dir", fskit.Entry("dir", fs.ModeDir|0755, now))
	fsys.SetNode("dir/file1.txt", fskit.Entry("file1.txt", 0644, []byte("content1"), now))
	fsys.SetNode("dir/file2.txt", fskit.Entry("file2.txt", 0644, []byte("content2"), now))
	fsys.SetNode("dir/subdir", fskit.Entry("subdir", fs.ModeDir|0755, now))
	fsys.SetNode("dir/subdir/file3.txt", fskit.Entry("file3.txt", 0644, []byte("content3"), now))

	// Test copying a directory within the same filesystem
	err := fs.CopyAll(fsys, "dir", "copydir")
	if err != nil {
		t.Fatalf("CopyAll failed: %v", err)
	}

	// Verify directory structure was copied
	testCases := []struct {
		path    string
		content string
		isDir   bool
	}{
		{"copydir", "", true},
		{"copydir/file1.txt", "content1", false},
		{"copydir/file2.txt", "content2", false},
		{"copydir/subdir", "", true},
		{"copydir/subdir/file3.txt", "content3", false},
	}

	for _, tc := range testCases {
		info, err := fs.Stat(fsys, tc.path)
		if err != nil {
			t.Errorf("Failed to stat %s: %v", tc.path, err)
			continue
		}

		if info.IsDir() != tc.isDir {
			t.Errorf("IsDir mismatch for %s: got %v, want %v", tc.path, info.IsDir(), tc.isDir)
		}

		if !tc.isDir && tc.content != "" {
			file, err := fsys.Open(tc.path)
			if err != nil {
				t.Errorf("Failed to open %s: %v", tc.path, err)
				continue
			}
			defer file.Close()

			buf := make([]byte, len(tc.content))
			n, err := file.Read(buf)
			if err != nil {
				t.Errorf("Failed to read %s: %v", tc.path, err)
				continue
			}
			if n != len(tc.content) || string(buf) != tc.content {
				t.Errorf("Content mismatch for %s: got %q, want %q", tc.path, string(buf), tc.content)
			}
		}
	}
}

func TestCopyAll_Symlink(t *testing.T) {
	// Create filesystem with a symlink
	fsys := memfs.New()
	now := time.Now()

	// Create target file and symlink
	fsys.SetNode("target.txt", fskit.Entry("target.txt", 0644, []byte("target content"), now))
	fsys.SetNode("link.txt", fskit.RawNode([]byte("target.txt"), fs.ModeSymlink|0777))

	// Test copying a symlink within the same filesystem
	err := fs.CopyAll(fsys, "link.txt", "copylink.txt")
	if err != nil {
		t.Fatalf("CopyAll failed: %v", err)
	}

	// Verify symlink was copied
	info, err := fs.Stat(fsys, "copylink.txt")
	if err != nil {
		t.Fatalf("Failed to stat copied symlink: %v", err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Error("Copied file is not a symlink")
	}

	// Verify symlink target
	target, err := fs.Readlink(fsys, "copylink.txt")
	if err != nil {
		t.Fatalf("Failed to read symlink target: %v", err)
	}
	if target != "target.txt" {
		t.Errorf("Symlink target mismatch: got %q, want %q", target, "target.txt")
	}
}

func TestCopyAll_OverwriteProtection(t *testing.T) {
	// Create filesystem with source and destination files
	fsys := memfs.New()
	now := time.Now()

	fsys.SetNode("test.txt", fskit.Entry("test.txt", 0644, []byte("source content"), now))
	fsys.SetNode("existing.txt", fskit.Entry("existing.txt", 0644, []byte("existing content"), now))

	// Test that CopyAll refuses to overwrite existing files
	err := fs.CopyAll(fsys, "test.txt", "existing.txt")
	if err == nil {
		t.Error("Expected error when trying to overwrite existing file, got nil")
	}
	if err != nil && err.Error() != `will not overwrite "existing.txt"` {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestCopyFS_CrossFilesystem(t *testing.T) {
	// Create separate source and destination filesystems
	srcFS := memfs.New()
	dstFS := memfs.New()
	now := time.Now()

	// Create test data in source
	content := []byte("cross-filesystem content")
	srcFS.SetNode("source.txt", fskit.Entry("source.txt", 0644, content, now))

	// Test copying between different filesystems
	err := fs.CopyFS(srcFS, "source.txt", dstFS, "dest.txt")
	if err != nil {
		t.Fatalf("CopyFS failed: %v", err)
	}

	// Verify the file was copied to destination filesystem
	dstFile, err := dstFS.Open("dest.txt")
	if err != nil {
		t.Fatalf("Failed to open copied file in destination: %v", err)
	}
	defer dstFile.Close()

	buf := make([]byte, len(content))
	n, err := dstFile.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if n != len(content) || string(buf) != string(content) {
		t.Errorf("Content mismatch: got %q, want %q", string(buf), string(content))
	}
}

func TestCopyNewFS_ModTimeCheck(t *testing.T) {
	// Create source and destination filesystems
	srcFS := memfs.New()
	dstFS := memfs.New()

	oldTime := time.Now().Add(-1 * time.Hour)
	newTime := time.Now()

	// Create source file with newer timestamp
	srcFS.SetNode("newer.txt", fskit.Entry("newer.txt", 0644, []byte("newer content"), newTime))

	// Create destination file with older timestamp
	dstFS.SetNode("older.txt", fskit.Entry("older.txt", 0644, []byte("older content"), oldTime))

	// Test copying newer file over older file
	err := fs.CopyNewFS(srcFS, "newer.txt", dstFS, "older.txt")
	if err != nil {
		t.Fatalf("CopyNewFS failed: %v", err)
	}

	// Verify the file was overwritten
	dstFile, err := dstFS.Open("older.txt")
	if err != nil {
		t.Fatalf("Failed to open destination file: %v", err)
	}
	defer dstFile.Close()

	buf := make([]byte, 100)
	n, err := dstFile.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}
	content := string(buf[:n])
	if content != "newer content" {
		t.Errorf("Content not updated: got %q, want %q", content, "newer content")
	}
}

func TestCopyNewFS_SkipOlderFile(t *testing.T) {
	// Create source and destination filesystems
	srcFS := memfs.New()
	dstFS := memfs.New()

	oldTime := time.Now().Add(-1 * time.Hour)
	newTime := time.Now()

	// Create source file with older timestamp
	srcFS.SetNode("older.txt", fskit.Entry("older.txt", 0644, []byte("older content"), oldTime))

	// Create destination file with newer timestamp
	dstFS.SetNode("newer.txt", fskit.Entry("newer.txt", 0644, []byte("newer content"), newTime))

	// Test copying older file over newer file (should be skipped)
	err := fs.CopyNewFS(srcFS, "older.txt", dstFS, "newer.txt")
	if err != nil {
		t.Fatalf("CopyNewFS failed: %v", err)
	}

	// Verify the file was NOT overwritten
	dstFile, err := dstFS.Open("newer.txt")
	if err != nil {
		t.Fatalf("Failed to open destination file: %v", err)
	}
	defer dstFile.Close()

	buf := make([]byte, 100)
	n, err := dstFile.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}
	content := string(buf[:n])
	if content != "newer content" {
		t.Errorf("Content was incorrectly updated: got %q, want %q", content, "newer content")
	}
}

func TestCopyNewFS_Directory(t *testing.T) {
	// Create source filesystem with directory structure
	srcFS := memfs.New()
	dstFS := memfs.New()

	oldTime := time.Now().Add(-1 * time.Hour)
	newTime := time.Now()

	// Create source directory with mixed file timestamps
	srcFS.SetNode("srcdir", fskit.Entry("srcdir", fs.ModeDir|0755, newTime))
	srcFS.SetNode("srcdir/newer.txt", fskit.Entry("newer.txt", 0644, []byte("newer content"), newTime))
	srcFS.SetNode("srcdir/older.txt", fskit.Entry("older.txt", 0644, []byte("older src content"), oldTime))

	// Create destination directory with existing files
	dstFS.SetNode("dstdir", fskit.Entry("dstdir", fs.ModeDir|0755, oldTime))
	dstFS.SetNode("dstdir/newer.txt", fskit.Entry("newer.txt", 0644, []byte("older dst content"), oldTime))
	dstFS.SetNode("dstdir/older.txt", fskit.Entry("older.txt", 0644, []byte("newer dst content"), newTime))

	// Test copying directory with CopyNewFS
	err := fs.CopyNewFS(srcFS, "srcdir", dstFS, "dstdir")
	if err != nil {
		t.Fatalf("CopyNewFS failed: %v", err)
	}

	// Verify newer.txt was overwritten (src is newer)
	file, err := dstFS.Open("dstdir/newer.txt")
	if err != nil {
		t.Fatalf("Failed to open newer.txt: %v", err)
	}
	defer file.Close()

	buf := make([]byte, 100)
	n, err := file.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read newer.txt: %v", err)
	}
	if string(buf[:n]) != "newer content" {
		t.Errorf("newer.txt not updated: got %q, want %q", string(buf[:n]), "newer content")
	}

	// Verify older.txt was NOT overwritten (dst is newer)
	file2, err := dstFS.Open("dstdir/older.txt")
	if err != nil {
		t.Fatalf("Failed to open older.txt: %v", err)
	}
	defer file2.Close()

	buf2 := make([]byte, 100)
	n2, err := file2.Read(buf2)
	if err != nil {
		t.Fatalf("Failed to read older.txt: %v", err)
	}
	if string(buf2[:n2]) != "newer dst content" {
		t.Errorf("older.txt was incorrectly updated: got %q, want %q", string(buf2[:n2]), "newer dst content")
	}
}

func TestCopyAll_NonExistentSource(t *testing.T) {
	fsys := memfs.New()

	err := fs.CopyAll(fsys, "nonexistent.txt", "dest.txt")
	if err == nil {
		t.Error("Expected error for nonexistent source file, got nil")
	}
}

func TestCopyAll_InvalidMode(t *testing.T) {
	fsys := memfs.New()
	now := time.Now()

	// Create a file with an unsupported mode (e.g., device file)
	fsys.SetNode("device", fskit.Entry("device", os.ModeDevice|0644, now))

	err := fs.CopyAll(fsys, "device", "copy_device")
	if err == nil {
		t.Error("Expected error for unsupported file mode, got nil")
	}
	if err != nil && !contains(err.Error(), "cannot copy file with mode") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestCopyFS_DestinationIsDirectory(t *testing.T) {
	srcFS := memfs.New()
	dstFS := memfs.New()
	now := time.Now()

	srcFS.SetNode("file.txt", fskit.Entry("file.txt", 0644, []byte("content"), now))
	dstFS.SetNode("existing_dir", fskit.Entry("existing_dir", fs.ModeDir|0755, now))

	// Should succeed when destination is a directory (copies into it)
	err := fs.CopyFS(srcFS, "file.txt", dstFS, "existing_dir")
	if err != nil {
		t.Fatalf("CopyFS should succeed when destination is directory: %v", err)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			indexOf(s, substr) >= 0)))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
