package metacache

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/memfs"
)

// trackingFS wraps a memfs and counts stat/readdir calls.
// It embeds memfs directly to inherit all its interface implementations.
type trackingFS struct {
	*memfs.FS
	statCalls    atomic.Int32
	readdirCalls atomic.Int32
}

func newTrackingFS() *trackingFS {
	return &trackingFS{FS: memfs.New()}
}

func (t *trackingFS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	t.statCalls.Add(1)
	return t.FS.StatContext(ctx, name)
}

func (t *trackingFS) ReadDirContext(ctx context.Context, name string) ([]fs.DirEntry, error) {
	t.readdirCalls.Add(1)
	entries, err := fs.ReadDirContext(ctx, t.FS, name)
	return entries, err
}

func newTestFS() (*trackingFS, *FS) {
	tracker := newTrackingFS()
	cached := New(tracker)
	return tracker, cached
}

func TestNew(t *testing.T) {
	mem := memfs.New()
	cached := New(mem)

	if cached == nil {
		t.Fatal("New returned nil")
	}
	if cached.GetTTL() != DefaultTTL {
		t.Errorf("expected default TTL %v, got %v", DefaultTTL, cached.GetTTL())
	}
}

func TestNewWithTTL(t *testing.T) {
	mem := memfs.New()
	ttl := 5 * time.Minute
	cached := NewWithTTL(mem, ttl)

	if cached.GetTTL() != ttl {
		t.Errorf("expected TTL %v, got %v", ttl, cached.GetTTL())
	}
}

func TestSetGetTTL(t *testing.T) {
	mem := memfs.New()
	cached := New(mem)

	newTTL := 10 * time.Second
	cached.SetTTL(newTTL)

	if cached.GetTTL() != newTTL {
		t.Errorf("expected TTL %v, got %v", newTTL, cached.GetTTL())
	}
}

func TestStatCaching(t *testing.T) {
	tracker, cached := newTestFS()

	// Create a file
	f, err := tracker.Create("test.txt")
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	f.Close()

	// First stat should hit the underlying FS
	info1, err := cached.Stat("test.txt")
	if err != nil {
		t.Fatalf("first Stat failed: %v", err)
	}
	if tracker.statCalls.Load() != 1 {
		t.Errorf("expected 1 stat call, got %d", tracker.statCalls.Load())
	}

	// Second stat should use cache
	info2, err := cached.Stat("test.txt")
	if err != nil {
		t.Fatalf("second Stat failed: %v", err)
	}
	if tracker.statCalls.Load() != 1 {
		t.Errorf("expected still 1 stat call (cached), got %d", tracker.statCalls.Load())
	}

	// Both should return same info
	if info1.Name() != info2.Name() || info1.Size() != info2.Size() {
		t.Error("cached info doesn't match original")
	}
}

func TestLstatCaching(t *testing.T) {
	tracker, cached := newTestFS()

	// Create a file
	f, err := tracker.Create("test.txt")
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	f.Close()

	// First lstat should hit the underlying FS
	_, err = cached.Lstat("test.txt")
	if err != nil {
		t.Fatalf("first Lstat failed: %v", err)
	}
	initialCalls := tracker.statCalls.Load()

	// Second lstat should use cache
	_, err = cached.Lstat("test.txt")
	if err != nil {
		t.Fatalf("second Lstat failed: %v", err)
	}
	if tracker.statCalls.Load() != initialCalls {
		t.Errorf("expected stat calls to stay at %d (cached), got %d", initialCalls, tracker.statCalls.Load())
	}
}

func TestReadDirCaching(t *testing.T) {
	tracker, cached := newTestFS()

	// Create some files
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		f, _ := tracker.Create(name)
		f.Close()
	}

	// First readdir should hit the underlying FS
	entries1, err := cached.ReadDirContext(context.Background(), ".")
	if err != nil {
		t.Fatalf("first ReadDir failed: %v", err)
	}
	if len(entries1) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries1))
	}
	if tracker.readdirCalls.Load() != 1 {
		t.Errorf("expected 1 readdir call, got %d", tracker.readdirCalls.Load())
	}

	// Second readdir should use cache
	entries2, err := cached.ReadDirContext(context.Background(), ".")
	if err != nil {
		t.Fatalf("second ReadDir failed: %v", err)
	}
	if len(entries2) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries2))
	}
	if tracker.readdirCalls.Load() != 1 {
		t.Errorf("expected still 1 readdir call (cached), got %d", tracker.readdirCalls.Load())
	}
}

func TestCacheExpiration(t *testing.T) {
	tracker, cached := newTestFS()

	// Set very short TTL for testing
	cached.SetTTL(50 * time.Millisecond)

	// Create a file
	f, _ := tracker.Create("test.txt")
	f.Close()

	// First stat
	_, err := cached.Stat("test.txt")
	if err != nil {
		t.Fatalf("first Stat failed: %v", err)
	}
	if tracker.statCalls.Load() != 1 {
		t.Errorf("expected 1 stat call, got %d", tracker.statCalls.Load())
	}

	// Wait for cache to expire
	time.Sleep(100 * time.Millisecond)

	// Stat should hit underlying FS again
	_, err = cached.Stat("test.txt")
	if err != nil {
		t.Fatalf("Stat after expiry failed: %v", err)
	}
	if tracker.statCalls.Load() != 2 {
		t.Errorf("expected 2 stat calls after expiry, got %d", tracker.statCalls.Load())
	}
}

func TestRefreshAhead(t *testing.T) {
	tracker, cached := newTestFS()

	// Set TTL so we can test refresh-ahead
	cached.SetTTL(100 * time.Millisecond)

	// Create a file
	f, _ := tracker.Create("test.txt")
	f.Close()

	// First stat
	_, err := cached.Stat("test.txt")
	if err != nil {
		t.Fatalf("first Stat failed: %v", err)
	}
	if tracker.statCalls.Load() != 1 {
		t.Errorf("expected 1 stat call, got %d", tracker.statCalls.Load())
	}

	// Wait until past renewsAt (TTL/2) but before expiresAt
	time.Sleep(60 * time.Millisecond)

	// This stat should return cached value but trigger async refresh
	_, err = cached.Stat("test.txt")
	if err != nil {
		t.Fatalf("Stat during refresh window failed: %v", err)
	}

	// Give the async refresh time to complete
	time.Sleep(50 * time.Millisecond)

	// Should have triggered a refresh
	if tracker.statCalls.Load() < 2 {
		t.Errorf("expected refresh-ahead to trigger, but stat calls = %d", tracker.statCalls.Load())
	}
}

func TestInvalidate(t *testing.T) {
	tracker, cached := newTestFS()

	// Create a file
	f, _ := tracker.Create("test.txt")
	f.Close()

	// Stat to cache
	_, _ = cached.Stat("test.txt")
	if tracker.statCalls.Load() != 1 {
		t.Errorf("expected 1 stat call, got %d", tracker.statCalls.Load())
	}

	// Invalidate
	cached.Invalidate("test.txt")

	// Stat should hit underlying FS again
	_, _ = cached.Stat("test.txt")
	if tracker.statCalls.Load() != 2 {
		t.Errorf("expected 2 stat calls after invalidation, got %d", tracker.statCalls.Load())
	}
}

func TestInvalidateDir(t *testing.T) {
	tracker, cached := newTestFS()

	// Create directory structure
	tracker.Mkdir("dir", 0755)
	f1, _ := tracker.Create("dir/a.txt")
	f1.Close()
	f2, _ := tracker.Create("dir/b.txt")
	f2.Close()

	// Cache all paths
	_, _ = cached.Stat("dir")
	_, _ = cached.Stat("dir/a.txt")
	_, _ = cached.Stat("dir/b.txt")
	initialCalls := tracker.statCalls.Load()

	// Invalidate directory
	cached.InvalidateDir("dir")

	// All paths should hit underlying FS again
	_, _ = cached.Stat("dir")
	_, _ = cached.Stat("dir/a.txt")
	_, _ = cached.Stat("dir/b.txt")

	expectedCalls := initialCalls + 3
	if tracker.statCalls.Load() != expectedCalls {
		t.Errorf("expected %d stat calls after dir invalidation, got %d", expectedCalls, tracker.statCalls.Load())
	}
}

func TestInvalidateAll(t *testing.T) {
	tracker, cached := newTestFS()

	// Create files
	f1, _ := tracker.Create("a.txt")
	f1.Close()
	f2, _ := tracker.Create("b.txt")
	f2.Close()

	// Cache
	_, _ = cached.Stat("a.txt")
	_, _ = cached.Stat("b.txt")
	initialCalls := tracker.statCalls.Load()

	// Invalidate all
	cached.InvalidateAll()

	// Both should hit underlying FS again
	_, _ = cached.Stat("a.txt")
	_, _ = cached.Stat("b.txt")

	expectedCalls := initialCalls + 2
	if tracker.statCalls.Load() != expectedCalls {
		t.Errorf("expected %d stat calls after invalidate all, got %d", expectedCalls, tracker.statCalls.Load())
	}
}

func TestErrorCaching(t *testing.T) {
	tracker, cached := newTestFS()

	// Stat non-existent file
	_, err := cached.Stat("nonexistent.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected ErrNotExist, got %v", err)
	}
	if tracker.statCalls.Load() != 1 {
		t.Errorf("expected 1 stat call, got %d", tracker.statCalls.Load())
	}

	// Second stat should return cached error
	_, err = cached.Stat("nonexistent.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected cached ErrNotExist, got %v", err)
	}
	if tracker.statCalls.Load() != 1 {
		t.Errorf("expected still 1 stat call (cached error), got %d", tracker.statCalls.Load())
	}
}

func TestMkdirInvalidatesParent(t *testing.T) {
	tracker, cached := newTestFS()

	// Cache root directory
	_, _ = cached.ReadDirContext(context.Background(), ".")
	initialCalls := tracker.readdirCalls.Load()

	// Mkdir should invalidate parent's readdir cache
	err := cached.Mkdir("newdir", 0755)
	if err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	// Check directory was actually created
	ok, _ := fs.Exists(tracker, "newdir")
	if !ok {
		t.Error("directory was not created")
	}

	// ReadDir should hit underlying FS again (cache was invalidated)
	_, _ = cached.ReadDirContext(context.Background(), ".")
	if tracker.readdirCalls.Load() <= initialCalls {
		t.Error("expected parent readdir cache to be invalidated")
	}
}

func TestRemoveInvalidates(t *testing.T) {
	tracker, cached := newTestFS()

	// Create file
	f, _ := tracker.Create("test.txt")
	f.Close()

	// Cache the file and parent
	_, _ = cached.Stat("test.txt")
	_, _ = cached.ReadDirContext(context.Background(), ".")
	statCalls := tracker.statCalls.Load()
	readdirCalls := tracker.readdirCalls.Load()

	// Remove should invalidate both
	err := cached.Remove("test.txt")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Stat should return not exist (file is gone)
	_, err = cached.Stat("test.txt")
	if err == nil {
		t.Error("expected error after remove, got nil")
	}
	if tracker.statCalls.Load() <= statCalls {
		t.Error("expected stat cache to be invalidated")
	}

	// ReadDir should show file is gone
	entries, _ := cached.ReadDirContext(context.Background(), ".")
	if tracker.readdirCalls.Load() <= readdirCalls {
		t.Error("expected readdir cache to be invalidated")
	}
	for _, e := range entries {
		if e.Name() == "test.txt" {
			t.Error("removed file still appears in directory listing")
		}
	}
}

func TestRenameInvalidates(t *testing.T) {
	tracker, cached := newTestFS()

	// Create file
	f, _ := tracker.Create("old.txt")
	f.Close()

	// Cache
	_, _ = cached.Stat("old.txt")
	statCalls := tracker.statCalls.Load()

	// Rename
	err := cached.Rename("old.txt", "new.txt")
	if err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	// Old path should be invalidated
	_, err = cached.Stat("old.txt")
	if err == nil {
		t.Error("old path should not exist")
	}

	// New path should work
	_, err = cached.Stat("new.txt")
	if err != nil {
		t.Errorf("new path Stat failed: %v", err)
	}

	// Should have made new stat calls
	if tracker.statCalls.Load() <= statCalls {
		t.Error("expected caches to be invalidated after rename")
	}
}

func TestChmodInvalidates(t *testing.T) {
	tracker, cached := newTestFS()

	// Create file
	f, _ := tracker.Create("test.txt")
	f.Close()

	// Cache
	_, _ = cached.Stat("test.txt")
	statCalls := tracker.statCalls.Load()

	// Chmod
	err := cached.Chmod("test.txt", 0755)
	if err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}

	// Cache should be invalidated - next stat should hit underlying FS
	_, _ = cached.Stat("test.txt")
	if tracker.statCalls.Load() <= statCalls {
		t.Error("expected cache to be invalidated after chmod")
	}
}

func TestFileWriteInvalidatesOnClose(t *testing.T) {
	tracker, cached := newTestFS()

	// Create file
	f, _ := tracker.Create("test.txt")
	f.Close()

	// Cache the file
	_, _ = cached.Stat("test.txt")
	statCalls := tracker.statCalls.Load()

	// Open file via cache and write to it
	file, err := cached.OpenFile("test.txt", 0x1, 0644) // O_WRONLY
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}

	// Write should be tracked - use io.Writer interface
	w, ok := file.(io.Writer)
	if !ok {
		t.Fatal("file does not implement io.Writer")
	}
	_, err = w.Write([]byte("new content"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Cache should still be valid before close
	_, _ = cached.Stat("test.txt")
	if tracker.statCalls.Load() != statCalls {
		t.Error("cache should not be invalidated before Close")
	}

	// Close should invalidate cache
	file.Close()

	// Next stat should hit underlying FS
	_, _ = cached.Stat("test.txt")
	if tracker.statCalls.Load() <= statCalls {
		t.Error("expected cache to be invalidated after Close with writes")
	}
}

func TestFileCloseWithoutWriteDoesNotInvalidate(t *testing.T) {
	tracker, cached := newTestFS()

	// Create file
	f, _ := tracker.Create("test.txt")
	f.Close()

	// Cache the file
	_, _ = cached.Stat("test.txt")
	statCalls := tracker.statCalls.Load()

	// Open file via cache (read-only, no write)
	file, err := cached.Open("test.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Close without writing
	file.Close()

	// Cache should still be valid
	_, _ = cached.Stat("test.txt")
	if tracker.statCalls.Load() != statCalls {
		t.Error("cache should not be invalidated after Close without writes")
	}
}

func TestCreateInvalidates(t *testing.T) {
	tracker, cached := newTestFS()

	// Cache root directory
	_, _ = cached.ReadDirContext(context.Background(), ".")
	readdirCalls := tracker.readdirCalls.Load()

	// Create new file
	f, err := cached.Create("newfile.txt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	f.Close()

	// Parent readdir cache should be invalidated
	entries, _ := cached.ReadDirContext(context.Background(), ".")
	if tracker.readdirCalls.Load() <= readdirCalls {
		t.Error("expected parent readdir cache to be invalidated")
	}

	// New file should appear
	found := false
	for _, e := range entries {
		if e.Name() == "newfile.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("created file not found in directory listing")
	}
}

func TestTruncateInvalidates(t *testing.T) {
	tracker, cached := newTestFS()

	// Create file with content
	err := fs.WriteFile(tracker, "test.txt", []byte("hello world"), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Cache
	info1, _ := cached.Stat("test.txt")
	if info1.Size() != 11 {
		t.Errorf("expected initial size 11, got %d", info1.Size())
	}
	statCalls := tracker.statCalls.Load()

	// Truncate
	err = cached.Truncate("test.txt", 5)
	if err != nil {
		t.Fatalf("Truncate failed: %v", err)
	}

	// Cache should be invalidated
	info2, _ := cached.Stat("test.txt")
	if tracker.statCalls.Load() <= statCalls {
		t.Error("expected cache to be invalidated after truncate")
	}

	if info2.Size() != 5 {
		t.Errorf("expected size 5 after truncate, got %d", info2.Size())
	}
}

func TestSymlinkInvalidates(t *testing.T) {
	tracker, cached := newTestFS()

	// Create target file
	f, _ := tracker.Create("target.txt")
	f.Close()

	// Cache parent
	_, _ = cached.ReadDirContext(context.Background(), ".")
	readdirCalls := tracker.readdirCalls.Load()

	// Create symlink
	err := cached.Symlink("target.txt", "link.txt")
	if err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	// Parent readdir cache should be invalidated
	_, _ = cached.ReadDirContext(context.Background(), ".")
	if tracker.readdirCalls.Load() <= readdirCalls {
		t.Error("expected parent readdir cache to be invalidated")
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "."},
		{".", "."},
		{"/", "."},
		{"/foo", "foo"},
		{"foo", "foo"},
		{"/foo/bar", "foo/bar"},
		{"foo/bar", "foo/bar"},
	}

	for _, tt := range tests {
		result := normalizePath(tt.input)
		if result != tt.expected {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	tracker, cached := newTestFS()

	// Create some files
	for i := 0; i < 10; i++ {
		f, _ := tracker.Create(string(rune('a'+i)) + ".txt")
		f.Close()
	}

	// Concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			for j := 0; j < 100; j++ {
				cached.Stat(string(rune('a'+idx)) + ".txt")
				cached.ReadDirContext(context.Background(), ".")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestConcurrentWriteInvalidate(t *testing.T) {
	tracker, cached := newTestFS()

	// Create initial file
	f, _ := tracker.Create("test.txt")
	f.Close()

	done := make(chan bool)

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			cached.Stat("test.txt")
		}
		done <- true
	}()

	// Concurrent invalidations
	go func() {
		for i := 0; i < 100; i++ {
			cached.Invalidate("test.txt")
		}
		done <- true
	}()

	// Wait for both
	<-done
	<-done
}

func TestRemoveAllInvalidatesRecursively(t *testing.T) {
	tracker, cached := newTestFS()

	// Create directory tree
	tracker.Mkdir("dir", 0755)
	f1, _ := tracker.Create("dir/a.txt")
	f1.Close()
	tracker.Mkdir("dir/subdir", 0755)
	f2, _ := tracker.Create("dir/subdir/b.txt")
	f2.Close()

	// Cache all paths
	_, _ = cached.Stat("dir")
	_, _ = cached.Stat("dir/a.txt")
	_, _ = cached.Stat("dir/subdir")
	_, _ = cached.Stat("dir/subdir/b.txt")
	statCalls := tracker.statCalls.Load()

	// RemoveAll
	err := cached.RemoveAll("dir")
	if err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}

	// All paths should be invalidated
	_, err = cached.Stat("dir")
	if err == nil {
		t.Error("dir should not exist after RemoveAll")
	}
	if tracker.statCalls.Load() <= statCalls {
		t.Error("expected cache to be invalidated after RemoveAll")
	}
}

func TestChownInvalidates(t *testing.T) {
	tracker, cached := newTestFS()

	// Create file
	f, _ := tracker.Create("test.txt")
	f.Close()

	// Cache
	_, _ = cached.Stat("test.txt")
	statCalls := tracker.statCalls.Load()

	// Chown
	err := cached.Chown("test.txt", 1000, 1000)
	if err != nil {
		t.Fatalf("Chown failed: %v", err)
	}

	// Cache should be invalidated
	_, _ = cached.Stat("test.txt")
	if tracker.statCalls.Load() <= statCalls {
		t.Error("expected cache to be invalidated after chown")
	}
}

func TestChtimesInvalidates(t *testing.T) {
	tracker, cached := newTestFS()

	// Create file
	f, _ := tracker.Create("test.txt")
	f.Close()

	// Cache
	_, _ = cached.Stat("test.txt")
	statCalls := tracker.statCalls.Load()

	// Chtimes
	newTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	err := cached.Chtimes("test.txt", newTime, newTime)
	if err != nil {
		t.Fatalf("Chtimes failed: %v", err)
	}

	// Cache should be invalidated - next stat should hit underlying FS
	_, _ = cached.Stat("test.txt")
	if tracker.statCalls.Load() <= statCalls {
		t.Error("expected cache to be invalidated after chtimes")
	}
}

func TestMkdirAllInvalidates(t *testing.T) {
	tracker, cached := newTestFS()

	// First, create parent "a" so we can cache its readdir
	err := tracker.Mkdir("a", 0755)
	if err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	// Cache the directory "a"
	_, _ = cached.ReadDirContext(context.Background(), "a")
	readdirCalls := tracker.readdirCalls.Load()

	// MkdirAll creates nested directories under "a"
	err = cached.MkdirAll("a/b/c", 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// Verify directories were created
	ok, _ := fs.Exists(tracker, "a/b/c")
	if !ok {
		t.Error("nested directories were not created")
	}

	// Cache for "a" should be invalidated - next readdir should hit underlying FS
	_, _ = cached.ReadDirContext(context.Background(), "a")
	if tracker.readdirCalls.Load() <= readdirCalls {
		t.Error("expected cache to be invalidated after MkdirAll")
	}
}

func TestStatAndLstatSeparateCaches(t *testing.T) {
	tracker, cached := newTestFS()

	// Create file
	f, _ := tracker.Create("test.txt")
	f.Close()

	// Stat and Lstat should have separate cache entries
	_, _ = cached.Stat("test.txt")
	statCalls := tracker.statCalls.Load()

	// Lstat should hit underlying FS (different cache key)
	_, _ = cached.Lstat("test.txt")
	if tracker.statCalls.Load() <= statCalls {
		t.Error("Lstat should use separate cache from Stat")
	}

	// Second Lstat should be cached
	lstatCalls := tracker.statCalls.Load()
	_, _ = cached.Lstat("test.txt")
	if tracker.statCalls.Load() != lstatCalls {
		t.Error("second Lstat should be cached")
	}
}

func TestFileStatUsesCachedInfo(t *testing.T) {
	tracker, cached := newTestFS()

	// Create file
	f, _ := tracker.Create("test.txt")
	f.Close()

	// Cache the file info via FS.Stat
	_, _ = cached.Stat("test.txt")
	statCalls := tracker.statCalls.Load()

	// Open file and call Stat on the file
	file, err := cached.Open("test.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer file.Close()

	// File.Stat should use cached info
	info, err := file.Stat()
	if err != nil {
		t.Fatalf("File.Stat failed: %v", err)
	}
	if info == nil {
		t.Fatal("File.Stat returned nil")
	}
	if tracker.statCalls.Load() != statCalls {
		t.Error("File.Stat should use cached info, not hit underlying FS")
	}
}

func TestFileReadDirUsesCachedEntries(t *testing.T) {
	tracker, cached := newTestFS()

	// Create some files
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		f, _ := tracker.Create(name)
		f.Close()
	}

	// Cache the directory via FS.ReadDirContext
	_, _ = cached.ReadDirContext(context.Background(), ".")
	readdirCalls := tracker.readdirCalls.Load()

	// Open directory and call ReadDir on the file
	file, err := cached.Open(".")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer file.Close()

	// Cast to ReadDirFile and call ReadDir
	rdf, ok := file.(fs.ReadDirFile)
	if !ok {
		t.Fatal("file does not implement ReadDirFile")
	}

	entries, err := rdf.ReadDir(-1)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// Should use cached entries, not hit underlying FS
	if tracker.readdirCalls.Load() != readdirCalls {
		t.Error("File.ReadDir should use cached entries, not hit underlying FS")
	}
}

func TestFileReadDirIterative(t *testing.T) {
	tracker, cached := newTestFS()

	// Create files
	for _, name := range []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt"} {
		f, _ := tracker.Create(name)
		f.Close()
	}

	// Open directory
	file, err := cached.Open(".")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer file.Close()

	rdf := file.(fs.ReadDirFile)

	// Read 2 entries at a time
	entries1, err := rdf.ReadDir(2)
	if err != nil {
		t.Fatalf("ReadDir(2) failed: %v", err)
	}
	if len(entries1) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries1))
	}

	entries2, err := rdf.ReadDir(2)
	if err != nil {
		t.Fatalf("ReadDir(2) second call failed: %v", err)
	}
	if len(entries2) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries2))
	}

	entries3, err := rdf.ReadDir(2)
	if err != nil {
		t.Fatalf("ReadDir(2) third call failed: %v", err)
	}
	if len(entries3) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries3))
	}

	// Next call should return EOF
	_, err = rdf.ReadDir(2)
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}
