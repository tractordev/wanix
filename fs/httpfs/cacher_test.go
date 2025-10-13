package httpfs

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"tractor.dev/wanix/fs/memfs"
)

// Helper function to create a test server with a cacher
func newTestServerWithCache() (*memfs.FS, *httptest.Server, *Cacher) {
	memFS := memfs.New()
	server := httptest.NewServer(NewServer(memFS))
	client := NewCacher(New(server.URL))
	return memFS, server, client
}

func TestCaching(t *testing.T) {
	_, server, cacher := newTestServerWithCache()
	defer server.Close()

	ctx := context.Background()

	// Create a file
	file, _ := cacher.CreateContext(ctx, "test.txt", []byte("cached content"), 0644)
	file.Close()

	// First stat should cache
	info1, err := cacher.Stat("test.txt")
	if err != nil {
		t.Fatalf("Failed to stat: %v", err)
	}

	// Second stat should use cache (we can't directly verify, but it should work)
	info2, err := cacher.Stat("test.txt")
	if err != nil {
		t.Fatalf("Failed to stat (cached): %v", err)
	}

	if info1.Size() != info2.Size() {
		t.Error("Cached info should match original")
	}

	// Test cache invalidation on modification
	if err := cacher.Chmod("test.txt", 0755); err != nil {
		t.Fatalf("Failed to chmod: %v", err)
	}

	// Stat should reflect the change
	info3, err := cacher.Stat("test.txt")
	if err != nil {
		t.Fatalf("Failed to stat after chmod: %v", err)
	}
	if info3.Mode()&0777 != 0755 {
		t.Errorf("Expected mode 0755, got %o", info3.Mode()&0777)
	}
}

func TestCacheTTL(t *testing.T) {
	_, server, cacher := newTestServerWithCache()
	defer server.Close()

	ctx := context.Background()

	// Set a very short TTL for testing
	cacher.SetTTL(100 * time.Millisecond)

	// Create a file
	file, _ := cacher.CreateContext(ctx, "test.txt", []byte("test"), 0644)
	file.Close()

	// First stat caches it
	_, err := cacher.Stat("test.txt")
	if err != nil {
		t.Fatalf("Failed to stat: %v", err)
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// This should trigger a refresh (but we can't directly verify without looking at requests)
	_, err = cacher.Stat("test.txt")
	if err != nil {
		t.Fatalf("Failed to stat after expiry: %v", err)
	}

	// Verify TTL getter/setter
	newTTL := 5 * time.Minute
	cacher.SetTTL(newTTL)
	if cacher.GetTTL() != newTTL {
		t.Errorf("Expected TTL %v, got %v", newTTL, cacher.GetTTL())
	}
}

func TestCacheInvalidation(t *testing.T) {
	_, server, cacher := newTestServerWithCache()
	defer server.Close()

	ctx := context.Background()

	// Create a file
	file, _ := cacher.CreateContext(ctx, "test.txt", []byte("original"), 0644)
	file.Close()

	// Cache it
	info1, _ := cacher.Stat("test.txt")
	originalSize := info1.Size()

	// Modify through cache (should invalidate)
	file, _ = cacher.CreateContext(ctx, "test.txt", []byte("modified content"), 0644)
	file.Close()

	// Next stat should see the change
	info2, _ := cacher.Stat("test.txt")
	if info2.Size() == originalSize {
		t.Error("Cache should have been invalidated after modification")
	}
}

func TestWriteFileWithCache(t *testing.T) {
	_, server, cacher := newTestServerWithCache()
	defer server.Close()

	// Write a file through cacher
	data := []byte("Cached WriteFile")
	err := cacher.WriteFile("cached.txt", data, 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// First stat should work
	info1, err := cacher.Stat("cached.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info1.Size() != int64(len(data)) {
		t.Errorf("Expected size %d, got %d", len(data), info1.Size())
	}

	// Overwrite with WriteFile
	newData := []byte("Updated via WriteFile")
	err = cacher.WriteFile("cached.txt", newData, 0644)
	if err != nil {
		t.Fatalf("WriteFile (overwrite) failed: %v", err)
	}

	// Stat should reflect the change (cache invalidated)
	info2, err := cacher.Stat("cached.txt")
	if err != nil {
		t.Fatalf("Stat after overwrite failed: %v", err)
	}
	if info2.Size() != int64(len(newData)) {
		t.Errorf("Expected size %d, got %d (cache not invalidated?)", len(newData), info2.Size())
	}
}
