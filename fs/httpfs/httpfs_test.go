package httpfs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHeadRequestCaching(t *testing.T) {
	requestCount := 0

	// Create a test server that counts requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", "1024")
			w.Header().Set("Content-Mode", "644")
			w.Header().Set("Content-Modified", "1609459200") // 2021-01-01
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	// Create filesystem with short cache TTL for testing
	fsys := New(server.URL)
	fsys.SetCacheTTL(100 * time.Millisecond)

	ctx := context.Background()
	testPath := "/test.txt"

	// First request should hit the server
	info1, err := fsys.headRequest(ctx, testPath)
	if err != nil {
		t.Fatalf("First headRequest failed: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request after first call, got %d", requestCount)
	}

	// Second request should use cache
	info2, err := fsys.headRequest(ctx, testPath)
	if err != nil {
		t.Fatalf("Second headRequest failed: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request after second call (cached), got %d", requestCount)
	}

	// Verify the cached result is the same
	if info1.Size() != info2.Size() || info1.ModTime() != info2.ModTime() {
		t.Error("Cached result differs from original")
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third request should hit the server again
	_, err = fsys.headRequest(ctx, testPath)
	if err != nil {
		t.Fatalf("Third headRequest failed: %v", err)
	}
	if requestCount != 2 {
		t.Errorf("Expected 2 requests after cache expiry, got %d", requestCount)
	}
}

func TestCacheInvalidation(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			requestCount++
			w.Header().Set("Content-Length", "1024")
			w.Header().Set("Content-Mode", "644")
			w.Header().Set("Content-Modified", "1609459200")
			w.WriteHeader(http.StatusOK)
		} else if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	fsys := New(server.URL)
	fsys.SetCacheTTL(1 * time.Hour) // Long TTL to ensure cache would normally persist

	ctx := context.Background()
	testPath := "/test.txt"

	// First request should hit the server and cache the result
	_, err := fsys.headRequest(ctx, testPath)
	if err != nil {
		t.Fatalf("First headRequest failed: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request after first call, got %d", requestCount)
	}

	// Second request should use cache
	_, err = fsys.headRequest(ctx, testPath)
	if err != nil {
		t.Fatalf("Second headRequest failed: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request after second call (cached), got %d", requestCount)
	}

	// Remove the file, which should invalidate the cache
	err = fsys.Remove(testPath)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Next HEAD request should hit the server again since cache was invalidated
	_, err = fsys.headRequest(ctx, testPath)
	if err != nil {
		t.Fatalf("Third headRequest failed: %v", err)
	}
	if requestCount != 2 {
		t.Errorf("Expected 2 requests after cache invalidation, got %d", requestCount)
	}
}

func TestCacheManagement(t *testing.T) {
	fsys := New("http://example.com")

	// Test TTL configuration
	newTTL := 10 * time.Minute
	fsys.SetCacheTTL(newTTL)

	if fsys.GetCacheTTL() != newTTL {
		t.Errorf("Expected TTL %v, got %v", newTTL, fsys.GetCacheTTL())
	}

	// Test cache clearing
	fsys.ClearHeadCache()

	// Add some mock cache entries for testing expiration
	now := time.Now()
	fsys.headCache["/expired"] = &headCacheEntry{
		info:      &httpFileInfo{name: "expired"},
		err:       nil,
		cachedAt:  now.Add(-1 * time.Hour),
		expiresAt: now.Add(-30 * time.Minute), // Expired
	}
	fsys.headCache["/valid"] = &headCacheEntry{
		info:      &httpFileInfo{name: "valid"},
		err:       nil,
		cachedAt:  now,
		expiresAt: now.Add(30 * time.Minute), // Valid
	}

	// Test expiration cleanup
	expired := fsys.ExpireOldHeadCache()
	if expired != 1 {
		t.Errorf("Expected 1 expired entry, got %d", expired)
	}

	// Verify only valid entry remains
	if len(fsys.headCache) != 1 {
		t.Errorf("Expected 1 cache entry remaining, got %d", len(fsys.headCache))
	}
	if _, exists := fsys.headCache["/valid"]; !exists {
		t.Error("Valid cache entry was incorrectly removed")
	}
}

func TestStatUsesCache(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			requestCount++
			w.Header().Set("Content-Length", "1024")
			w.Header().Set("Content-Mode", "644")
			w.Header().Set("Content-Modified", "1609459200")
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	fsys := New(server.URL)
	testPath := "/test.txt"

	// First Stat call should hit the server
	_, err := fsys.Stat(testPath)
	if err != nil {
		t.Fatalf("First Stat failed: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request after first Stat, got %d", requestCount)
	}

	// Second Stat call should use cache
	_, err = fsys.Stat(testPath)
	if err != nil {
		t.Fatalf("Second Stat failed: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request after second Stat (cached), got %d", requestCount)
	}

	// StatContext should also use cache
	_, err = fsys.StatContext(context.Background(), testPath)
	if err != nil {
		t.Fatalf("StatContext failed: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request after StatContext (cached), got %d", requestCount)
	}
}

func TestHeadRequest404Caching(t *testing.T) {
	requestCount := 0

	// Create a test server that always returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	// Create filesystem with short cache TTL for testing
	fsys := New(server.URL)
	fsys.SetCacheTTL(100 * time.Millisecond)

	ctx := context.Background()
	testPath := "/nonexistent.txt"

	// First request should hit the server and get 404
	_, err := fsys.headRequest(ctx, testPath)
	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}
	if err.Error() != "file does not exist" {
		t.Errorf("Expected fs.ErrNotExist, got %v", err)
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request after first call, got %d", requestCount)
	}

	// Second request should use cached 404 error
	_, err = fsys.headRequest(ctx, testPath)
	if err == nil {
		t.Fatal("Expected cached error for non-existent file")
	}
	if err.Error() != "file does not exist" {
		t.Errorf("Expected cached fs.ErrNotExist, got %v", err)
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request after second call (cached error), got %d", requestCount)
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third request should hit the server again
	_, err = fsys.headRequest(ctx, testPath)
	if err == nil {
		t.Fatal("Expected error after cache expiry")
	}
	if requestCount != 2 {
		t.Errorf("Expected 2 requests after cache expiry, got %d", requestCount)
	}
}

func TestStat404Caching(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			requestCount++
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	fsys := New(server.URL)
	testPath := "/nonexistent.txt"

	// First Stat call should hit the server and get 404
	_, err := fsys.Stat(testPath)
	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request after first Stat, got %d", requestCount)
	}

	// Second Stat call should use cached 404 error
	_, err = fsys.Stat(testPath)
	if err == nil {
		t.Fatal("Expected cached error for non-existent file")
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request after second Stat (cached error), got %d", requestCount)
	}

	// StatContext should also use cached 404 error
	_, err = fsys.StatContext(context.Background(), testPath)
	if err == nil {
		t.Fatal("Expected cached error for StatContext")
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request after StatContext (cached error), got %d", requestCount)
	}
}

func Test404CacheInvalidation(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			requestCount++
			w.WriteHeader(http.StatusNotFound)
		} else if r.Method == "PUT" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	fsys := New(server.URL)
	fsys.SetCacheTTL(1 * time.Hour) // Long TTL to ensure cache would normally persist

	ctx := context.Background()
	testPath := "/test.txt"

	// First request should hit the server and cache the 404 error
	_, err := fsys.headRequest(ctx, testPath)
	if err == nil {
		t.Fatal("Expected 404 error")
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request after first call, got %d", requestCount)
	}

	// Second request should use cached 404 error
	_, err = fsys.headRequest(ctx, testPath)
	if err == nil {
		t.Fatal("Expected cached 404 error")
	}
	if requestCount != 1 {
		t.Errorf("Expected 1 request after second call (cached error), got %d", requestCount)
	}

	// Create the file, which should invalidate the cached 404 error
	_, err = fsys.Create(testPath)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Next HEAD request should hit the server again since cache was invalidated
	// (This will still return 404 in our test server, but the point is that it makes a new request)
	_, err = fsys.headRequest(ctx, testPath)
	if err == nil {
		t.Fatal("Expected 404 error from server")
	}
	if requestCount != 2 {
		t.Errorf("Expected 2 requests after cache invalidation, got %d", requestCount)
	}
}
