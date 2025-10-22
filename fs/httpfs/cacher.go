package httpfs

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
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

func (fsys *Cacher) unwrap() *FS {
	return fsys.fs
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
func (fsys *Cacher) CachedNode(path string) (*Node, error, bool) {
	path = normalizePath(path)
	fsys.cacheMu.RLock()
	entry, exists := fsys.nodeCache[path]
	fsys.cacheMu.RUnlock()
	if !exists {
		fsys.log.Debug("Node miss", "path", path)
		return nil, nil, false
	}

	if time.Now().After(entry.expiresAt) {
		fsys.log.Debug("Node expired", "path", path, "expired", time.Since(entry.expiresAt))
		fsys.cacheMu.Lock()
		delete(fsys.nodeCache, path)
		fsys.cacheMu.Unlock()
		return nil, nil, false
	}

	if !entry.renewsAt.IsZero() && time.Now().After(entry.renewsAt) {
		fsys.log.Debug("Node renew", "path", path)
		if entry.node.IsDir() {
			go fsys.PullDir(context.Background(), path)
		} else {
			go fsys.PullMeta(context.Background(), path)
		}
	}

	// If this is a cached error, return it
	if entry.err != nil {
		return nil, entry.err, true
	}

	return entry.node, nil, true
}

// CacheNode stores node result in cache with appropriate TTL
func (fsys *Cacher) CacheNode(node *Node) {
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()

	now := time.Now()

	fsys.log.Debug("CacheNode", "path", node.Path())
	fsys.nodeCache[node.Path()] = &cacheEntry{
		node:      node,
		err:       nil,
		cachedAt:  now,
		expiresAt: now.Add(fsys.ttl),
		renewsAt:  now.Add(fsys.ttl / 2),
	}
}

// CacheNodeError stores node request error in cache
func (fsys *Cacher) CacheNodeError(path string, err error) {
	path = normalizePath(path)
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()

	now := time.Now()
	// Use file TTL for errors by default
	fsys.nodeCache[path] = &cacheEntry{
		node:      nil,
		err:       err,
		cachedAt:  now,
		expiresAt: now.Add(fsys.ttl / 2),
	}
}

// InvalidateNode invalidates a node in the cache
func (fsys *Cacher) InvalidateNode(path string, resync, recursive bool) error {
	path = normalizePath(path)
	fsys.log.Debug("InvalidateNode", "path", path, "resync", resync, "recursive", recursive)

	fsys.cacheMu.Lock()
	entry, exists := fsys.nodeCache[path]
	delete(fsys.nodeCache, path)
	fsys.cacheMu.Unlock()

	if recursive && exists && entry.node != nil && entry.node.IsDir() {
		for _, entry := range entry.node.Entries() {
			fsys.InvalidateNode(filepath.Join(path, entry.Name()), false, true)
		}
	}

	if resync {
		_, err := fsys.PullDir(context.Background(), filepath.Dir(path))
		return err
	}

	return nil
}

// InvalidateAll clears all cached node results
func (fsys *Cacher) InvalidateAll(name string) {
	name = normalizePath(name)
	fsys.log.Debug("InvalidateAll", "name", name)
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()
	if name == "." || name == "" {
		fsys.nodeCache = make(map[string]*cacheEntry)
	} else {
		delete(fsys.nodeCache, name)
		for key := range fsys.nodeCache {
			if strings.HasPrefix(key, name+"/") {
				delete(fsys.nodeCache, key)
			}
		}
	}
}

// SetTTL sets the cache TTL for directory nodes
func (fsys *Cacher) SetTTL(ttl time.Duration) {
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()
	fsys.ttl = ttl
}

// GetTTL returns the current directory cache TTL
func (fsys *Cacher) GetTTL() time.Duration {
	fsys.cacheMu.RLock()
	defer fsys.cacheMu.RUnlock()
	return fsys.ttl
}

// ============================================================================
// Pull Methods (Fetch and Cache)
// ============================================================================

// PullNode fetches and caches a node with optional subtree prefetching
// Called by Open and ReadDir on cache miss.
func (fsys *Cacher) PullNode(ctx context.Context, name string, recursivePrefetch bool) (*Node, []byte, error) {
	name = normalizePath(name)
	fsys.log.Debug("PullNode", "name", name, "rp", recursivePrefetch)

	// Use the underlying FS to open the file
	file, err := fsys.fs.OpenContext(ctx, name)
	if err != nil {
		if err == fs.ErrNotExist {
			fsys.CacheNodeError(name, fs.ErrNotExist)
		}
		return nil, nil, err
	}
	defer file.Close()

	// Get the file info and content
	node, ok := file.(*Node)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected file type")
	}

	fsys.CacheNode(node)

	if !node.IsDir() || !recursivePrefetch {
		return node, node.content, nil
	}

	// Start async prefetch for subdirectories
	for _, entry := range node.Entries() {
		if entry.IsDir() {
			go func(path string) {
				for n, err := range fsys.fs.streamTree(ctx, path) {
					if err != nil {
						fsys.log.Debug("incomplete prefetch", "path", path, "err", err)
						return
					}
					// Cache the directory node
					fsys.CacheNode(n)
				}
			}(filepath.Join(name, entry.Name()))
		}
	}

	return node, nil, nil
}

// PullDir gets metadata for a directory and its entries using multipart response.
// Called async on cache lookup w/ renewal, called sync on cache invalidate w/ resync.
// Called sync on StatContext w/ cache miss.
func (fsys *Cacher) PullDir(ctx context.Context, name string) (*Node, error) {
	name = normalizePath(name)
	fsys.log.Debug("PullDir", "name", name)
	url := fsys.fs.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "multipart/mixed")
	resp, err := fsys.fs.doRequest(req)
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
	for fileNode, err := range parseNodesMultipart(fsys, resp.Body, boundary) {
		if err != nil {
			return nil, err
		}
		fsys.CacheNode(fileNode)
		if dirNode == nil {
			dirNode = fileNode
		}
	}
	return dirNode, nil
}

// PullMeta performs a HEAD request to get metadata.
// Called async on cache lookup w/ renewal, called sync on StatContext w/ cache miss.
func (fsys *Cacher) PullMeta(ctx context.Context, path string) (*Node, error) {
	path = normalizePath(path)
	fsys.log.Debug("PullMeta", "path", path)

	node, err := fsys.fs.StatContext(ctx, path)
	if err != nil {
		if err == fs.ErrNotExist {
			// Cache the 404 error
			fsys.CacheNodeError(path, fs.ErrNotExist)
		}
		return nil, err
	}

	fileNode, ok := node.(*Node)
	if !ok {
		return nil, fmt.Errorf("unexpected node type")
	}

	// Cache the result
	fsys.CacheNode(fileNode)

	return fileNode, nil
}

// ============================================================================
// Read Operations (with caching)
// ============================================================================

// Open opens the named file for reading
func (fsys *Cacher) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

// OpenContext opens the named file for reading with context
func (fsys *Cacher) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys.log.Debug("Open", "name", name)
	// Always get a fresh file handle from the underlying FS
	// Don't use PullNode here because it closes the file after caching
	// We need an unclosed file handle for read/write operations
	file, err := fsys.fs.OpenContext(ctx, name)
	if err != nil {
		if err == fs.ErrNotExist {
			fsys.CacheNodeError(name, fs.ErrNotExist)
		}
		return nil, err
	}

	// Cache the node metadata for future Stat calls, but return the open file
	if node, ok := file.(*Node); ok {
		node.fs = fsys
		fsys.CacheNode(node)
	}

	return file, nil
}

// Stat returns file information
func (fsys *Cacher) Stat(name string) (fs.FileInfo, error) {
	return fsys.StatContext(context.Background(), name)
}

// StatContext performs a HEAD request to get file metadata if not cached
func (fsys *Cacher) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	if cachedNode, cachedErr, found := fsys.CachedNode(name); found {
		fsys.log.Debug("Stat", "name", name, "cached", found)
		if cachedErr != nil {
			return nil, cachedErr
		}
		return cachedNode, nil
	}
	fsys.log.Debug("Stat", "name", name, "cached", false)

	// special case for attribute nodes
	if strings.Contains(name, "/:attr") {
		f, err := fsys.OpenContext(ctx, name)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return f.Stat()
	}

	var isDir bool
	if name == "." {
		isDir = true
	} else {
		if parent, _, exists := fsys.CachedNode(filepath.Dir(name)); exists {
			for _, entry := range parent.Entries() {
				if entry.Name() == filepath.Base(name) {
					isDir = entry.Type().IsDir()
					break
				}
			}
		}
	}

	if isDir {
		return fsys.PullDir(ctx, name)
	} else {
		n, err := fsys.PullMeta(ctx, name)
		if err != nil {
			return nil, err
		}
		if n.IsDir() {
			// path and parent path expired, so had to
			// pull meta to know its a directory
			return fsys.PullDir(ctx, name)
		}
		return n, nil
	}
}

// ReadDir reads the named directory and returns a list of directory entries
func (fsys *Cacher) ReadDir(name string) ([]fs.DirEntry, error) {
	return fsys.ReadDirContext(context.Background(), name)
}

// ReadDirContext reads the named directory with context
func (fsys *Cacher) ReadDirContext(ctx context.Context, name string) ([]fs.DirEntry, error) {
	// check if we have a cached directory node
	if cachedNode, cachedErr, found := fsys.CachedNode(name); found {
		fsys.log.Debug("ReadDir", "name", name, "cached", found)
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
	fsys.log.Debug("ReadDir", "name", name, "cached", false)

	node, _, err := fsys.PullNode(ctx, name, true)
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
func (fsys *Cacher) Readlink(name string) (string, error) {
	return fsys.ReadlinkContext(context.Background(), name)
}

// ReadlinkContext reads the value of a symbolic link with context
func (fsys *Cacher) ReadlinkContext(ctx context.Context, name string) (string, error) {
	return fsys.fs.ReadlinkContext(ctx, name)
}

// ============================================================================
// Write Operations
// ============================================================================

// WriteFile writes data to the named file, creating it if necessary
func (fsys *Cacher) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return fsys.WriteFileContext(context.Background(), name, data, perm, time.Now())
}

// WriteFileContext writes data to the named file with context
func (fsys *Cacher) WriteFileContext(ctx context.Context, name string, data []byte, perm fs.FileMode, modTime time.Time) error {
	if err := fsys.fs.WriteFileContext(ctx, name, data, perm, modTime); err != nil {
		return err
	}
	return fsys.InvalidateNode(name, true, false)
}

// Create creates or truncates the named file
func (fsys *Cacher) Create(name string) (fs.File, error) {
	return fsys.CreateContext(context.Background(), name, nil, 0644)
}

// CreateContext is a helper for creating files with content and mode
func (fsys *Cacher) CreateContext(ctx context.Context, name string, content []byte, mode fs.FileMode) (fs.File, error) {
	file, err := fsys.fs.CreateContext(ctx, name, content, mode)
	if err != nil {
		return nil, err
	}

	if err := fsys.InvalidateNode(name, true, false); err != nil {
		return nil, err
	}

	return file, nil
}

// Symlink creates a symbolic link
func (fsys *Cacher) Symlink(oldname, newname string) error {
	return fsys.SymlinkContext(context.Background(), oldname, newname)
}

// SymlinkContext creates a symbolic link with context
func (fsys *Cacher) SymlinkContext(ctx context.Context, oldname, newname string) error {
	if err := fsys.fs.SymlinkContext(ctx, oldname, newname); err != nil {
		return err
	}
	return fsys.InvalidateNode(newname, true, false)
}

// Rename renames a file or directory
func (fsys *Cacher) Rename(oldname, newname string) error {
	return fsys.RenameContext(context.Background(), oldname, newname)
}

// RenameContext renames a file or directory with context
func (fsys *Cacher) RenameContext(ctx context.Context, oldname, newname string) error {
	if err := fsys.fs.RenameContext(ctx, oldname, newname); err != nil {
		return err
	}

	if err := fsys.InvalidateNode(newname, true, false); err != nil {
		return err
	}
	return fsys.InvalidateNode(oldname, true, true)
}

// Mkdir creates a directory
func (fsys *Cacher) Mkdir(name string, perm fs.FileMode) error {
	return fsys.MkdirContext(context.Background(), name, perm)
}

// MkdirContext creates a directory with context
func (fsys *Cacher) MkdirContext(ctx context.Context, name string, perm fs.FileMode) error {
	if err := fsys.fs.MkdirContext(ctx, name, perm); err != nil {
		return err
	}
	return fsys.InvalidateNode(name, true, false)
}

// Remove removes a file or empty directory
func (fsys *Cacher) Remove(name string) error {
	return fsys.RemoveContext(context.Background(), name)
}

// RemoveContext removes a file or directory with context
func (fsys *Cacher) RemoveContext(ctx context.Context, name string) error {
	if err := fsys.fs.RemoveContext(ctx, name); err != nil {
		return err
	}
	return fsys.InvalidateNode(name, true, true)
}

// Chmod changes the mode of the named file
func (fsys *Cacher) Chmod(name string, mode fs.FileMode) error {
	return fsys.ChmodContext(context.Background(), name, mode)
}

// ChmodContext changes file mode with context
func (fsys *Cacher) ChmodContext(ctx context.Context, name string, mode fs.FileMode) error {
	if err := fsys.fs.ChmodContext(ctx, name, mode); err != nil {
		return err
	}
	return fsys.InvalidateNode(name, false, false)
}

// Chown changes the numeric uid and gid of the named file
func (fsys *Cacher) Chown(name string, uid, gid int) error {
	return fsys.ChownContect(context.Background(), name, uid, gid)
}

// ChownContect changes ownership with context
func (fsys *Cacher) ChownContect(ctx context.Context, name string, uid, gid int) error {
	if err := fsys.fs.ChownContect(ctx, name, uid, gid); err != nil {
		return err
	}
	return fsys.InvalidateNode(name, false, false)
}

// Chtimes changes the access and modification times of the named file
func (fsys *Cacher) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return fsys.ChtimesContext(context.Background(), name, atime, mtime)
}

// ChtimesContext changes times with context
func (fsys *Cacher) ChtimesContext(ctx context.Context, name string, atime time.Time, mtime time.Time) error {
	if err := fsys.fs.ChtimesContext(ctx, name, atime, mtime); err != nil {
		return err
	}
	return fsys.InvalidateNode(name, false, false)
}

// Patch applies a tar patch to the filesystem
func (fsys *Cacher) Patch(ctx context.Context, name string, tarBuf bytes.Buffer) error {
	if err := fsys.fs.Patch(ctx, name, tarBuf); err != nil {
		return err
	}
	fsys.InvalidateAll(name)
	return nil
}
