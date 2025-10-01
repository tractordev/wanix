package readthru

import (
	"context"
	"fmt"
	"sync"
	"time"

	"tractor.dev/wanix/fs"
)

// FetchFunc is the function signature for fetching content from the source
type FetchFunc func(ctx context.Context, path string) (content []byte, info fs.FileInfo, err error)

// StoreFunc is the function signature for storing content in the cache
type StoreFunc func(path string, content []byte) error

// Metadata holds cache metadata for files
type Metadata struct {
	CachedAt      time.Time
	ExpiryAt      time.Time
	RemoteModTime time.Time
	RemoteSize    int64
}

// Cache manages read-through caching with configurable TTL
type Cache struct {
	cacheTTL  time.Duration
	fetchFunc FetchFunc
	storeFunc StoreFunc
	metadata  map[string]*Metadata
	mutex     sync.RWMutex
}

// New creates a new read-through cache
func New(cacheTTL time.Duration, fetchFunc FetchFunc, storeFunc StoreFunc) *Cache {
	return &Cache{
		cacheTTL:  cacheTTL,
		fetchFunc: fetchFunc,
		storeFunc: storeFunc,
		metadata:  make(map[string]*Metadata),
	}
}

// Get retrieves content from cache or fetches from source
func (c *Cache) Get(ctx context.Context, path string, openCached func(path string) (fs.File, error)) (fs.File, error) {
	c.mutex.RLock()

	// Check if cache is valid
	if c.isValid(path) {
		c.mutex.RUnlock()

		// Try to open from cache
		file, err := openCached(path)
		if err == nil {
			return file, nil
		}

		// Cache file missing, fall through to fetch
		c.mutex.Lock()
		c.invalidate(path)
		c.mutex.Unlock()
	} else {
		c.mutex.RUnlock()
	}

	// Cache miss or invalid - fetch from source
	return c.fetchAndCache(ctx, path, openCached)
}

// fetchAndCache retrieves content from source and caches it
func (c *Cache) fetchAndCache(ctx context.Context, path string, openCached func(path string) (fs.File, error)) (fs.File, error) {
	// Fetch content from source
	content, sourceInfo, err := c.fetchFunc(ctx, path)
	if err != nil {
		return nil, err
	}

	// Store in cache
	err = c.storeFunc(path, content)
	if err != nil {
		return nil, fmt.Errorf("store in cache %s: %w", path, err)
	}

	// Update metadata
	now := time.Now()
	metadata := &Metadata{
		CachedAt:      now,
		ExpiryAt:      now.Add(c.cacheTTL),
		RemoteModTime: sourceInfo.ModTime(),
		RemoteSize:    sourceInfo.Size(),
	}

	c.mutex.Lock()
	c.metadata[path] = metadata
	c.mutex.Unlock()

	// Open cached file
	return openCached(path)
}

// isValid checks if a cached file is still valid (caller must hold read lock)
func (c *Cache) isValid(path string) bool {
	metadata, exists := c.metadata[path]
	if !exists {
		return false
	}

	return time.Now().Before(metadata.ExpiryAt)
}

// IsValid checks if a cached file is still valid (thread-safe)
func (c *Cache) IsValid(path string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.isValid(path)
}

// Invalidate removes a file from the cache
func (c *Cache) Invalidate(path string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.invalidate(path)
}

// invalidate removes metadata (caller must hold write lock)
func (c *Cache) invalidate(path string) {
	delete(c.metadata, path)
}

// Status returns cache status for a file
func (c *Cache) Status(path string) (cached bool, valid bool, expiresAt time.Time, err error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	metadata, exists := c.metadata[path]
	if !exists {
		return false, false, time.Time{}, nil
	}

	cached = true
	valid = time.Now().Before(metadata.ExpiryAt)
	expiresAt = metadata.ExpiryAt

	return cached, valid, expiresAt, nil
}

// SetTTL updates the cache TTL
func (c *Cache) SetTTL(ttl time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cacheTTL = ttl
}

// GetTTL returns the current cache TTL
func (c *Cache) GetTTL() time.Duration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.cacheTTL
}

// CachedCount returns the number of cached files
func (c *Cache) CachedCount() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.metadata)
}

// Clear removes all cached metadata
func (c *Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.metadata = make(map[string]*Metadata)
}

// ExpireOld removes expired entries from metadata
func (c *Cache) ExpireOld() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	expired := 0

	for path, metadata := range c.metadata {
		if now.After(metadata.ExpiryAt) {
			delete(c.metadata, path)
			expired++
		}
	}

	return expired
}

// GetMetadata returns the metadata for a cached file
func (c *Cache) GetMetadata(path string) (*Metadata, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	metadata, exists := c.metadata[path]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modification
	metadataCopy := *metadata
	return &metadataCopy, true
}
