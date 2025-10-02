package cachefs

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

func TestCacheFS_ReadThrough(t *testing.T) {
	// Setup local and remote filesystems
	local := fskit.MemFS{}
	remote := fskit.MemFS{
		"test.txt":     fskit.RawNode([]byte("hello from remote")),
		"dir/file.txt": fskit.RawNode([]byte("nested file")),
	}

	// Create cache filesystem
	cfs := New(local, remote, time.Minute, nil)
	defer cfs.Close()

	// Test reading file that doesn't exist in cache
	file, err := cfs.Open("test.txt")
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	content, err := fs.ReadFile(cfs, "test.txt")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != "hello from remote" {
		t.Errorf("Expected 'hello from remote', got %q", string(content))
	}

	// Verify file is now cached locally
	localContent, err := fs.ReadFile(local, "test.txt")
	if err != nil {
		t.Fatalf("File should be cached locally: %v", err)
	}

	if string(localContent) != "hello from remote" {
		t.Errorf("Cached content mismatch: expected 'hello from remote', got %q", string(localContent))
	}

	// Test reading nested file
	nestedContent, err := fs.ReadFile(cfs, "dir/file.txt")
	if err != nil {
		t.Fatalf("Failed to read nested file: %v", err)
	}

	if string(nestedContent) != "nested file" {
		t.Errorf("Expected 'nested file', got %q", string(nestedContent))
	}
}

func TestCacheFS_WriteBack(t *testing.T) {
	// Setup filesystems
	local := fskit.MemFS{}
	remote := fskit.MemFS{}

	// Create cache filesystem with error logging
	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)
	cfs := New(local, remote, time.Minute, logger)
	defer cfs.Close()

	// Test creating a new file
	file, err := cfs.Create("newfile.txt")
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Write content
	content := []byte("hello world")
	_, err = fs.Write(file, content)
	if err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}

	// Close file to trigger async write
	err = file.Close()
	if err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}

	// Wait for async operations to complete
	cfs.Wait()

	// Verify file exists in local filesystem
	localContent, err := fs.ReadFile(local, "newfile.txt")
	if err != nil {
		t.Fatalf("File should exist in local filesystem: %v", err)
	}

	if string(localContent) != "hello world" {
		t.Errorf("Local content mismatch: expected 'hello world', got %q", string(localContent))
	}

	// Verify file exists in remote filesystem
	remoteContent, err := fs.ReadFile(remote, "newfile.txt")
	if err != nil {
		t.Fatalf("File should exist in remote filesystem: %v", err)
	}

	if string(remoteContent) != "hello world" {
		t.Errorf("Remote content mismatch: expected 'hello world', got %q", string(remoteContent))
	}

	// Check that no errors were logged
	if logBuf.Len() > 0 {
		t.Errorf("Unexpected errors logged: %s", logBuf.String())
	}
}

func TestCacheFS_DirectoryOperations(t *testing.T) {
	// Setup filesystems
	local := fskit.MemFS{}
	remote := fskit.MemFS{}

	cfs := New(local, remote, time.Minute, nil)
	defer cfs.Close()

	// Test creating directory
	err := cfs.Mkdir("testdir", 0755)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Wait for async operations
	cfs.Wait()

	// Verify directory exists in both filesystems
	localExists, err := fs.DirExists(local, "testdir")
	if err != nil {
		t.Fatalf("Error checking local directory: %v", err)
	}
	if !localExists {
		t.Error("Directory should exist in local filesystem")
	}

	remoteExists, err := fs.DirExists(remote, "testdir")
	if err != nil {
		t.Fatalf("Error checking remote directory: %v", err)
	}
	if !remoteExists {
		t.Error("Directory should exist in remote filesystem")
	}

	// Test removing directory
	err = cfs.Remove("testdir")
	if err != nil {
		t.Fatalf("Failed to remove directory: %v", err)
	}

	// Wait for async operations
	cfs.Wait()

	// Verify directory is removed from both filesystems
	localExists, _ = fs.DirExists(local, "testdir")
	if localExists {
		t.Error("Directory should be removed from local filesystem")
	}

	remoteExists, _ = fs.DirExists(remote, "testdir")
	if remoteExists {
		t.Error("Directory should be removed from remote filesystem")
	}
}

func TestCacheFS_MetadataOperations(t *testing.T) {
	// Setup filesystems with initial file
	local := fskit.MemFS{}
	remote := fskit.MemFS{
		"test.txt": fskit.RawNode([]byte("test content"), fs.FileMode(0644)),
	}

	cfs := New(local, remote, time.Minute, nil)
	defer cfs.Close()

	// First read the file to cache it
	_, err := fs.ReadFile(cfs, "test.txt")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Test chmod
	err = cfs.Chmod("test.txt", 0755)
	if err != nil {
		t.Fatalf("Failed to chmod: %v", err)
	}

	// Wait for async operations
	cfs.Wait()

	// Verify mode changed in local filesystem
	localInfo, err := fs.Stat(local, "test.txt")
	if err != nil {
		t.Fatalf("Failed to stat local file: %v", err)
	}

	if localInfo.Mode().Perm() != 0755 {
		t.Errorf("Local file mode should be 0755, got %o", localInfo.Mode().Perm())
	}

	// Verify mode changed in remote filesystem
	remoteInfo, err := fs.Stat(remote, "test.txt")
	if err != nil {
		t.Fatalf("Failed to stat remote file: %v", err)
	}

	if remoteInfo.Mode().Perm() != 0755 {
		t.Errorf("Remote file mode should be 0755, got %o", remoteInfo.Mode().Perm())
	}
}

func TestCacheFS_CacheInvalidation(t *testing.T) {
	// Setup filesystems
	local := fskit.MemFS{}
	remote := fskit.MemFS{
		"test.txt": fskit.RawNode([]byte("original content")),
	}

	cfs := New(local, remote, time.Minute, nil)
	defer cfs.Close()

	// Read file to cache it
	content1, err := fs.ReadFile(cfs, "test.txt")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content1) != "original content" {
		t.Errorf("Expected 'original content', got %q", string(content1))
	}

	// Modify the file through the cache filesystem
	err = fs.WriteFile(cfs, "test.txt", []byte("modified content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Wait for async operations
	cfs.Wait()

	// Read again - should get the modified content
	content2, err := fs.ReadFile(cfs, "test.txt")
	if err != nil {
		t.Fatalf("Failed to read file after modification: %v", err)
	}

	if string(content2) != "modified content" {
		t.Errorf("Expected 'modified content', got %q", string(content2))
	}

	// Verify remote was updated
	remoteContent, err := fs.ReadFile(remote, "test.txt")
	if err != nil {
		t.Fatalf("Failed to read remote file: %v", err)
	}

	if string(remoteContent) != "modified content" {
		t.Errorf("Remote should have 'modified content', got %q", string(remoteContent))
	}
}

func TestCacheFS_ErrorHandling(t *testing.T) {
	// Setup filesystems where remote doesn't support certain operations
	local := fskit.MemFS{}
	remote := fskit.MemFS{} // MemFS supports all operations, so we'll use a read-only wrapper

	// Create a read-only wrapper for remote
	readOnlyRemote := &readOnlyFS{remote}

	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)
	cfs := New(local, readOnlyRemote, time.Minute, logger)
	defer cfs.Close()

	// Create a file directly to test write operations
	file, err := cfs.Create("test.txt")
	if err != nil {
		t.Fatalf("Create should succeed: %v", err)
	}

	// Write content
	_, err = fs.Write(file, []byte("test content"))
	if err != nil {
		t.Fatalf("Write should succeed: %v", err)
	}

	// Close file to trigger async write to remote (which should fail)
	err = file.Close()
	if err != nil {
		t.Fatalf("Close should succeed: %v", err)
	}

	// Wait for async operations to complete
	cfs.Wait()

	// Give a small delay for the error logging goroutine to process
	time.Sleep(10 * time.Millisecond)

	// Check that an error was logged
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "async write error") {
		t.Errorf("Expected async write error to be logged, got: %s", logOutput)
	}

	// Verify file exists locally
	localExists, err := fs.Exists(local, "test.txt")
	if err != nil {
		t.Fatalf("Error checking local file: %v", err)
	}
	if !localExists {
		t.Error("File should exist locally")
	}

	// Verify file does NOT exist in remote (due to read-only wrapper)
	remoteExists, _ := fs.Exists(readOnlyRemote, "test.txt")
	if remoteExists {
		t.Error("File should not exist in remote filesystem due to write failure")
	}
}

func TestCacheFS_CacheTTL(t *testing.T) {
	// Setup filesystems
	local := fskit.MemFS{}
	remote := fskit.MemFS{
		"test.txt": fskit.RawNode([]byte("original")),
	}

	// Create cache with very short TTL
	cfs := New(local, remote, 10*time.Millisecond, nil)
	defer cfs.Close()

	// Read file to cache it
	content1, err := fs.ReadFile(cfs, "test.txt")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content1) != "original" {
		t.Errorf("Expected 'original', got %q", string(content1))
	}

	// Modify remote file directly (bypassing cache)
	remote["test.txt"] = fskit.RawNode([]byte("modified"))

	// Read immediately - should still get cached version
	content2, err := fs.ReadFile(cfs, "test.txt")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content2) != "original" {
		t.Errorf("Should still get cached version, got %q", string(content2))
	}

	// Wait for cache to expire
	time.Sleep(20 * time.Millisecond)

	// Read again - should get updated version from remote
	content3, err := fs.ReadFile(cfs, "test.txt")
	if err != nil {
		t.Fatalf("Failed to read file after cache expiry: %v", err)
	}

	if string(content3) != "modified" {
		t.Errorf("Should get updated version after cache expiry, got %q", string(content3))
	}
}

// readOnlyFS is a wrapper that makes a filesystem read-only for testing error handling
type readOnlyFS struct {
	fs.FS
}

// Implement fs.CreateFS interface to return permission error
func (rofs *readOnlyFS) Create(name string) (fs.File, error) {
	return nil, fs.ErrPermission
}

func (rofs *readOnlyFS) Mkdir(name string, perm fs.FileMode) error {
	return fs.ErrPermission
}

func (rofs *readOnlyFS) Remove(name string) error {
	return fs.ErrPermission
}

func (rofs *readOnlyFS) Chmod(name string, mode fs.FileMode) error {
	return fs.ErrPermission
}

func (rofs *readOnlyFS) Chown(name string, uid, gid int) error {
	return fs.ErrPermission
}

func (rofs *readOnlyFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return fs.ErrPermission
}

func (rofs *readOnlyFS) Rename(oldname, newname string) error {
	return fs.ErrPermission
}

func (rofs *readOnlyFS) Symlink(oldname, newname string) error {
	return fs.ErrPermission
}
