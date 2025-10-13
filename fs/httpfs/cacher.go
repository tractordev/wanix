package httpfs

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"iter"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// cacheEntry represents a cached node result
type cacheEntry struct {
	node      *Node
	err       error // stores cached errors (e.g., fs.ErrNotExist for 404)
	cachedAt  time.Time
	renewsAt  time.Time
	expiresAt time.Time
}

// Cacher wraps an FS and adds caching functionality
type Cacher struct {
	fs        *FS
	nodeCache map[string]*cacheEntry
	cacheMu   sync.RWMutex
	ttl       time.Duration // TTL for cached nodes
	log       *slog.Logger
}

// NewCacher creates a new caching wrapper around an FS
func NewCacher(fs *FS) *Cacher {
	return &Cacher{
		fs:        fs,
		nodeCache: make(map[string]*cacheEntry),
		ttl:       20 * time.Second,
		log:       fs.log,
	}
}

func (c *Cacher) unwrap() *FS {
	return c.fs
}

// normalizePath ensures consistent path format for cache keys
func normalizePath(p string) string {
	// Remove leading slash to match Node.Path() behavior
	return strings.TrimPrefix(p, "/")
}

// ============================================================================
// Cache Management Methods
// ============================================================================

// CachedNode retrieves cached node result if valid
func (c *Cacher) CachedNode(path string) (*Node, error, bool) {
	path = normalizePath(path)
	c.cacheMu.RLock()
	entry, exists := c.nodeCache[path]
	c.cacheMu.RUnlock()
	if !exists {
		c.log.Debug("Node miss", "path", path)
		return nil, nil, false
	}

	if time.Now().After(entry.expiresAt) {
		c.log.Debug("Node expired", "path", path, "expired", time.Since(entry.expiresAt))
		c.cacheMu.Lock()
		delete(c.nodeCache, path)
		c.cacheMu.Unlock()
		return nil, nil, false
	}

	if !entry.renewsAt.IsZero() && time.Now().After(entry.renewsAt) {
		c.log.Debug("Node renew", "path", path)
		if entry.node.IsDir() {
			go c.PullDir(context.Background(), path)
		} else {
			go c.PullMeta(context.Background(), path)
		}
	}

	// If this is a cached error, return it
	if entry.err != nil {
		return nil, entry.err, true
	}

	return entry.node, nil, true
}

// CacheNode stores node result in cache with appropriate TTL
func (c *Cacher) CacheNode(node *Node) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	now := time.Now()

	c.log.Debug("CacheNode", "path", node.Path())
	c.nodeCache[node.Path()] = &cacheEntry{
		node:      node,
		err:       nil,
		cachedAt:  now,
		expiresAt: now.Add(c.ttl),
		renewsAt:  now.Add(c.ttl / 2),
	}
}

// CacheNodeError stores node request error in cache
func (c *Cacher) CacheNodeError(path string, err error) {
	path = normalizePath(path)
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	now := time.Now()
	// Use file TTL for errors by default
	c.nodeCache[path] = &cacheEntry{
		node:      nil,
		err:       err,
		cachedAt:  now,
		expiresAt: now.Add(c.ttl / 2),
	}
}

// InvalidateNode invalidates a node in the cache
func (c *Cacher) InvalidateNode(path string, resync, recursive bool) error {
	path = normalizePath(path)
	c.log.Debug("InvalidateNode", "path", path, "resync", resync, "recursive", recursive)

	c.cacheMu.Lock()
	entry, exists := c.nodeCache[path]
	delete(c.nodeCache, path)
	c.cacheMu.Unlock()

	if recursive && exists && entry.node != nil && entry.node.IsDir() {
		for _, entry := range entry.node.Entries() {
			c.InvalidateNode(filepath.Join(path, entry.Name()), false, true)
		}
	}

	if resync {
		_, err := c.PullDir(context.Background(), filepath.Dir(path))
		return err
	}

	return nil
}

// InvalidateAll clears all cached node results
func (c *Cacher) InvalidateAll(name string) {
	name = normalizePath(name)
	c.log.Debug("InvalidateAll", "name", name)
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	if name == "." || name == "" {
		c.nodeCache = make(map[string]*cacheEntry)
	} else {
		delete(c.nodeCache, name)
		for key := range c.nodeCache {
			if strings.HasPrefix(key, name+"/") {
				delete(c.nodeCache, key)
			}
		}
	}
}

// SetTTL sets the cache TTL for directory nodes
func (c *Cacher) SetTTL(ttl time.Duration) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.ttl = ttl
}

// GetTTL returns the current directory cache TTL
func (c *Cacher) GetTTL() time.Duration {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	return c.ttl
}

// ============================================================================
// Pull Methods (Fetch and Cache)
// ============================================================================

// PullNode fetches and caches a node with optional subtree prefetching
// Called by Open and ReadDir on cache miss.
func (c *Cacher) PullNode(ctx context.Context, name string, recursivePrefetch bool) (*Node, []byte, error) {
	name = normalizePath(name)
	c.log.Debug("PullNode", "name", name, "rp", recursivePrefetch)

	// Use the underlying FS to open the file
	file, err := c.fs.OpenContext(ctx, name)
	if err != nil {
		if err == fs.ErrNotExist {
			c.CacheNodeError(name, fs.ErrNotExist)
		}
		return nil, nil, err
	}
	defer file.Close()

	// Get the file info and content
	node, ok := file.(*Node)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected file type")
	}

	c.CacheNode(node)

	if !node.IsDir() || !recursivePrefetch {
		return node, node.content, nil
	}

	// Start async prefetch for subdirectories
	for _, entry := range node.Entries() {
		if entry.IsDir() {
			go func(path string) {
				for n, err := range c.streamTree(ctx, path) {
					if err != nil {
						c.log.Debug("incomplete prefetch", "path", path, "err", err)
						return
					}
					// Cache the directory node
					c.CacheNode(n)
				}
			}(filepath.Join(name, entry.Name()))
		}
	}

	return node, nil, nil
}

// PullDir gets metadata for a directory and its entries using multipart response.
// Called async on cache lookup w/ renewal, called sync on cache invalidate w/ resync.
// Called sync on StatContext w/ cache miss.
func (c *Cacher) PullDir(ctx context.Context, name string) (*Node, error) {
	name = normalizePath(name)
	c.log.Debug("PullDir", "name", name)
	url := c.fs.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "multipart/mixed")
	resp, err := c.fs.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Check if response is multipart
	contentType := resp.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return nil, fmt.Errorf("expected multipart response, got %s", contentType)
	}

	boundary := params["boundary"]
	if boundary == "" {
		return nil, fmt.Errorf("no boundary in multipart response")
	}

	// Parse multipart response
	var dirNode *Node
	for fileNode, err := range parseNodesMultipart(c, resp.Body, boundary) {
		if err != nil {
			return nil, err
		}
		c.CacheNode(fileNode)
		if dirNode == nil {
			dirNode = fileNode
		}
	}
	return dirNode, nil
}

// PullMeta performs a HEAD request to get metadata.
// Called async on cache lookup w/ renewal, called sync on StatContext w/ cache miss.
func (c *Cacher) PullMeta(ctx context.Context, path string) (*Node, error) {
	path = normalizePath(path)
	c.log.Debug("PullMeta", "path", path)

	node, err := c.fs.StatContext(ctx, path)
	if err != nil {
		if err == fs.ErrNotExist {
			// Cache the 404 error
			c.CacheNodeError(path, fs.ErrNotExist)
		}
		return nil, err
	}

	fileNode, ok := node.(*Node)
	if !ok {
		return nil, fmt.Errorf("unexpected node type")
	}

	// Cache the result
	c.CacheNode(fileNode)

	return fileNode, nil
}

// streamTree fetches a directory tree using multipart response
func (c *Cacher) streamTree(ctx context.Context, name string) iter.Seq2[*Node, error] {
	return func(yield func(*Node, error) bool) {
		// Request the directory tree with "..." suffix for streaming recursive multipart response
		url := c.fs.buildURL(name + "/...")

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			yield(nil, err)
			return
		}

		req.Header.Set("Accept", "multipart/mixed")
		resp, err := c.fs.doRequest(req)
		if err != nil {
			yield(nil, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			yield(nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status))
			return
		}

		// Check if response is multipart
		contentType := resp.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
			yield(nil, fmt.Errorf("expected multipart response, got %s", contentType))
			return
		}

		boundary := params["boundary"]
		if boundary == "" {
			yield(nil, fmt.Errorf("no boundary in multipart response"))
			return
		}

		// Parse multipart response
		for fileNode, err := range parseNodesMultipart(c, resp.Body, boundary) {
			if err != nil {
				yield(nil, err)
				return
			}
			if !fileNode.IsDir() {
				continue
			}
			if !yield(fileNode, nil) {
				return
			}
		}
	}
}

// ============================================================================
// Read Operations (with caching)
// ============================================================================

// Open opens the named file for reading
func (c *Cacher) Open(name string) (fs.File, error) {
	return c.OpenContext(context.Background(), name)
}

// OpenContext opens the named file for reading with context
func (c *Cacher) OpenContext(ctx context.Context, name string) (fs.File, error) {
	c.log.Debug("Open", "name", name)
	// Always get a fresh file handle from the underlying FS
	// Don't use PullNode here because it closes the file after caching
	// We need an unclosed file handle for read/write operations
	file, err := c.fs.OpenContext(ctx, name)
	if err != nil {
		if err == fs.ErrNotExist {
			c.CacheNodeError(name, fs.ErrNotExist)
		}
		return nil, err
	}

	// Cache the node metadata for future Stat calls, but return the open file
	if node, ok := file.(*Node); ok {
		node.fs = c
		c.CacheNode(node)
	}

	return file, nil
}

// Stat returns file information
func (c *Cacher) Stat(name string) (fs.FileInfo, error) {
	return c.StatContext(context.Background(), name)
}

// StatContext performs a HEAD request to get file metadata if not cached
func (c *Cacher) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	if cachedNode, cachedErr, found := c.CachedNode(name); found {
		c.log.Debug("Stat", "name", name, "cached", found)
		if cachedErr != nil {
			return nil, cachedErr
		}
		return cachedNode, nil
	}
	c.log.Debug("Stat", "name", name, "cached", false)

	var isDir bool
	if name == "." {
		isDir = true
	} else {
		if parent, _, exists := c.CachedNode(filepath.Dir(name)); exists {
			for _, entry := range parent.Entries() {
				if entry.Name() == filepath.Base(name) {
					isDir = entry.Type().IsDir()
					break
				}
			}
		}
	}

	if isDir {
		return c.PullDir(ctx, name)
	} else {
		n, err := c.PullMeta(ctx, name)
		if err != nil {
			return nil, err
		}
		if n.IsDir() {
			// path and parent path expired, so had to
			// pull meta to know its a directory
			return c.PullDir(ctx, name)
		}
		return n, nil
	}
}

// ReadDir reads the named directory and returns a list of directory entries
func (c *Cacher) ReadDir(name string) ([]fs.DirEntry, error) {
	return c.ReadDirContext(context.Background(), name)
}

// ReadDirContext reads the named directory with context
func (c *Cacher) ReadDirContext(ctx context.Context, name string) ([]fs.DirEntry, error) {
	// check if we have a cached directory node
	if cachedNode, cachedErr, found := c.CachedNode(name); found {
		c.log.Debug("ReadDir", "name", name, "cached", found)
		if cachedErr != nil {
			return nil, cachedErr
		}
		if !cachedNode.IsDir() {
			return nil, fmt.Errorf("not a directory")
		}
		// Convert fileNode entries to fs.DirEntry
		entries := make([]fs.DirEntry, len(cachedNode.Entries()))
		for i, entry := range cachedNode.Entries() {
			entries[i] = entry
		}
		return entries, nil
	}
	c.log.Debug("ReadDir", "name", name, "cached", false)

	node, _, err := c.PullNode(ctx, name, true)
	if err != nil {
		return nil, err
	}

	if !node.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}

	// Convert fileNode entries to fs.DirEntry
	entries := make([]fs.DirEntry, len(node.Entries()))
	for i, entry := range node.Entries() {
		entries[i] = entry
	}
	return entries, nil
}

// Readlink reads the value of a symbolic link
func (c *Cacher) Readlink(name string) (string, error) {
	return c.ReadlinkContext(context.Background(), name)
}

// ReadlinkContext reads the value of a symbolic link with context
func (c *Cacher) ReadlinkContext(ctx context.Context, name string) (string, error) {
	return c.fs.ReadlinkContext(ctx, name)
}

// ============================================================================
// Write Operations (with invalidation)
// ============================================================================

// WriteFile writes data to the named file, creating it if necessary
func (c *Cacher) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return c.WriteFileContext(context.Background(), name, data, perm, time.Now())
}

// WriteFileContext writes data to the named file with context
func (c *Cacher) WriteFileContext(ctx context.Context, name string, data []byte, perm fs.FileMode, modTime time.Time) error {
	if err := c.fs.WriteFileContext(ctx, name, data, perm, modTime); err != nil {
		return err
	}
	return c.InvalidateNode(name, true, false)
}

// Create creates or truncates the named file
func (c *Cacher) Create(name string) (fs.File, error) {
	return c.CreateContext(context.Background(), name, nil, 0644)
}

// CreateContext is a helper for creating files with content and mode
func (c *Cacher) CreateContext(ctx context.Context, name string, content []byte, mode fs.FileMode) (fs.File, error) {
	file, err := c.fs.CreateContext(ctx, name, content, mode)
	if err != nil {
		return nil, err
	}

	if err := c.InvalidateNode(name, true, false); err != nil {
		return nil, err
	}

	return file, nil
}

// Symlink creates a symbolic link
func (c *Cacher) Symlink(oldname, newname string) error {
	return c.SymlinkContext(context.Background(), oldname, newname)
}

// SymlinkContext creates a symbolic link with context
func (c *Cacher) SymlinkContext(ctx context.Context, oldname, newname string) error {
	if err := c.fs.SymlinkContext(ctx, oldname, newname); err != nil {
		return err
	}
	return c.InvalidateNode(newname, true, false)
}

// Rename renames a file or directory
func (c *Cacher) Rename(oldname, newname string) error {
	return c.RenameContext(context.Background(), oldname, newname)
}

// RenameContext renames a file or directory with context
func (c *Cacher) RenameContext(ctx context.Context, oldname, newname string) error {
	if err := c.fs.RenameContext(ctx, oldname, newname); err != nil {
		return err
	}

	if err := c.InvalidateNode(newname, true, false); err != nil {
		return err
	}
	return c.InvalidateNode(oldname, true, true)
}

// Mkdir creates a directory
func (c *Cacher) Mkdir(name string, perm fs.FileMode) error {
	return c.MkdirContext(context.Background(), name, perm)
}

// MkdirContext creates a directory with context
func (c *Cacher) MkdirContext(ctx context.Context, name string, perm fs.FileMode) error {
	if err := c.fs.MkdirContext(ctx, name, perm); err != nil {
		return err
	}
	return c.InvalidateNode(name, true, false)
}

// Remove removes a file or empty directory
func (c *Cacher) Remove(name string) error {
	return c.RemoveContext(context.Background(), name)
}

// RemoveContext removes a file or directory with context
func (c *Cacher) RemoveContext(ctx context.Context, name string) error {
	if err := c.fs.RemoveContext(ctx, name); err != nil {
		return err
	}
	return c.InvalidateNode(name, true, true)
}

// Chmod changes the mode of the named file
func (c *Cacher) Chmod(name string, mode fs.FileMode) error {
	return c.ChmodContext(context.Background(), name, mode)
}

// ChmodContext changes file mode with context
func (c *Cacher) ChmodContext(ctx context.Context, name string, mode fs.FileMode) error {
	if err := c.fs.ChmodContext(ctx, name, mode); err != nil {
		return err
	}
	return c.InvalidateNode(name, false, false)
}

// Chown changes the numeric uid and gid of the named file
func (c *Cacher) Chown(name string, uid, gid int) error {
	return c.ChownContect(context.Background(), name, uid, gid)
}

// ChownContect changes ownership with context
func (c *Cacher) ChownContect(ctx context.Context, name string, uid, gid int) error {
	if err := c.fs.ChownContect(ctx, name, uid, gid); err != nil {
		return err
	}
	return c.InvalidateNode(name, false, false)
}

// Chtimes changes the access and modification times of the named file
func (c *Cacher) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return c.ChtimesContext(context.Background(), name, atime, mtime)
}

// ChtimesContext changes times with context
func (c *Cacher) ChtimesContext(ctx context.Context, name string, atime time.Time, mtime time.Time) error {
	if err := c.fs.ChtimesContext(ctx, name, atime, mtime); err != nil {
		return err
	}
	return c.InvalidateNode(name, false, false)
}

// ApplyPatch applies a tar patch to the filesystem
func (c *Cacher) ApplyPatch(name string, tarBuf bytes.Buffer) error {
	if err := c.fs.ApplyPatch(name, tarBuf); err != nil {
		return err
	}
	c.InvalidateAll(name)
	return nil
}
