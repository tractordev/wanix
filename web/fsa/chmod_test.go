//go:build js && wasm

package fsa

import (
	"testing"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

// TestChmodPersistence tests that chmod permissions persist after cache expiry
func TestChmodPersistence(t *testing.T) {
	// Create a mock FS for testing
	fsys := &FS{
		statCache: make(map[string]*statCacheEntry),
		cacheTTL:  50 * time.Millisecond, // Short TTL for testing
	}

	// Initialize metadata store with a mock OPFS root
	mockRoot := &FS{
		statCache: make(map[string]*statCacheEntry),
		cacheTTL:  50 * time.Millisecond,
	}
	mockRoot.opfsRoot = mockRoot
	fsys.opfsRoot = mockRoot

	// Initialize metadata store
	metadataStore := GetMetadataStore()
	metadataStore.opfsRoot = mockRoot

	testPath := "test.txt"

	// Set initial metadata with default permissions
	metadataStore.SetMetadata(testPath, FileMetadata{
		Mode:  DefaultFileMode,
		Mtime: time.Now(),
		Atime: time.Now(),
	})

	// Verify initial permissions
	metadata, exists := metadataStore.GetMetadata(testPath)
	if !exists {
		t.Fatal("Initial metadata should exist")
	}
	if metadata.Mode != DefaultFileMode {
		t.Errorf("Initial mode should be %o, got %o", DefaultFileMode, metadata.Mode)
	}

	// Change permissions via chmod
	newMode := fs.FileMode(0755)
	err := fsys.Chmod(testPath, newMode)
	if err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}

	// Verify permissions were updated in metadata store
	metadata, exists = metadataStore.GetMetadata(testPath)
	if !exists {
		t.Fatal("Metadata should still exist after chmod")
	}
	if metadata.Mode != newMode {
		t.Errorf("Mode should be %o after chmod, got %o", newMode, metadata.Mode)
	}

	// Wait for cache to expire
	time.Sleep(100 * time.Millisecond)

	// Verify permissions are still correct after cache expiry
	metadata, exists = metadataStore.GetMetadata(testPath)
	if !exists {
		t.Fatal("Metadata should persist after cache expiry")
	}
	if metadata.Mode != newMode {
		t.Errorf("Mode should still be %o after cache expiry, got %o", newMode, metadata.Mode)
	}
}

// TestMetadataStore tests the basic functionality of the metadata store
func TestMetadataStore(t *testing.T) {
	// Get a fresh metadata store instance
	store := &MetadataStore{}

	testPath := "test/file.txt"
	testMode := fs.FileMode(0644)
	testTime := time.Now()

	// Test SetMode
	store.SetMode(testPath, testMode)

	metadata, exists := store.GetMetadata(testPath)
	if !exists {
		t.Fatal("Metadata should exist after SetMode")
	}
	if metadata.Mode != testMode {
		t.Errorf("Mode should be %o, got %o", testMode, metadata.Mode)
	}

	// Test SetTimes
	atime := testTime.Add(-time.Hour)
	mtime := testTime.Add(-30 * time.Minute)
	store.SetTimes(testPath, atime, mtime)

	metadata, exists = store.GetMetadata(testPath)
	if !exists {
		t.Fatal("Metadata should exist after SetTimes")
	}
	if metadata.Mode != testMode {
		t.Errorf("Mode should be preserved: %o, got %o", testMode, metadata.Mode)
	}
	if !metadata.Atime.Equal(atime) {
		t.Errorf("Atime should be %v, got %v", atime, metadata.Atime)
	}
	if !metadata.Mtime.Equal(mtime) {
		t.Errorf("Mtime should be %v, got %v", mtime, metadata.Mtime)
	}

	// Test DeleteMetadata
	store.DeleteMetadata(testPath)
	_, exists = store.GetMetadata(testPath)
	if exists {
		t.Error("Metadata should not exist after deletion")
	}
}

// TestStatCaching tests the stat caching functionality
func TestStatCaching(t *testing.T) {
	fsys := &FS{
		statCache: make(map[string]*statCacheEntry),
		cacheTTL:  100 * time.Millisecond,
	}

	testPath := "cached_file.txt"
	testInfo := fskit.Entry("cached_file.txt", 0644, 1024, time.Now())

	// Cache a stat result
	fsys.setCachedStat(testPath, testInfo)

	// Verify it's cached
	info, err, found := fsys.getCachedStat(testPath)
	if !found {
		t.Fatal("Stat should be cached")
	}
	if err != nil {
		t.Fatalf("Cached stat should not have error: %v", err)
	}
	if info.Name() != testInfo.Name() {
		t.Errorf("Cached name should be %s, got %s", testInfo.Name(), info.Name())
	}
	if info.Mode() != testInfo.Mode() {
		t.Errorf("Cached mode should be %o, got %o", testInfo.Mode(), info.Mode())
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Verify cache expired
	_, _, found = fsys.getCachedStat(testPath)
	if found {
		t.Error("Stat should not be cached after expiry")
	}

	// Test error caching
	testErr := fs.ErrNotExist
	fsys.setCachedStatError(testPath, testErr)

	_, err, found = fsys.getCachedStat(testPath)
	if !found {
		t.Fatal("Error should be cached")
	}
	if err != testErr {
		t.Errorf("Cached error should be %v, got %v", testErr, err)
	}

	// Test cache invalidation
	fsys.invalidateCachedStat(testPath)
	_, _, found = fsys.getCachedStat(testPath)
	if found {
		t.Error("Stat should not be cached after invalidation")
	}
}
