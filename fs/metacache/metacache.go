package metacache

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tractor.dev/wanix/fs"
)

const (
	DefaultTTL = 30 * time.Second
)

// cacheEntry represents a cached metadata result
type cacheEntry struct {
	info      fs.FileInfo   // Cached stat result (nil if entries is set)
	entries   []fs.DirEntry // Cached ReadDir result (nil if info is set)
	err       error         // Cached error (e.g., fs.ErrNotExist)
	cachedAt  time.Time     // When the entry was cached
	renewsAt  time.Time     // Halfway point - trigger refresh-ahead
	expiresAt time.Time     // Hard expiry - must refetch
}

// FS wraps a filesystem and caches metadata (FileInfo/DirEntry) with
// configurable TTL, refresh-ahead at halfway through expiry, and
// explicit invalidation methods.
type FS struct {
	*fs.DefaultFS
	cache   map[string]*cacheEntry
	cacheMu sync.RWMutex
	ttl     time.Duration
	log     *slog.Logger
}

// New wraps an fs.FS with metadata caching using the default TTL.
func New(fsys fs.FS) *FS {
	return NewWithTTL(fsys, DefaultTTL)
}

// NewWithTTL wraps an fs.FS with metadata caching using the specified TTL.
func NewWithTTL(fsys fs.FS, ttl time.Duration) *FS {
	return &FS{
		DefaultFS: fs.NewDefault(fsys),
		cache:     make(map[string]*cacheEntry),
		ttl:       ttl,
		log:       slog.Default(),
	}
}

// SetLogger sets the logger for cache operations.
func (f *FS) SetLogger(log *slog.Logger) {
	f.log = log
}

// ============================================================================
// TTL Management
// ============================================================================

// SetTTL sets the cache TTL. Only affects new entries.
func (f *FS) SetTTL(ttl time.Duration) {
	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()
	f.ttl = ttl
}

// GetTTL returns the current cache TTL.
func (f *FS) GetTTL() time.Duration {
	f.cacheMu.RLock()
	defer f.cacheMu.RUnlock()
	return f.ttl
}

// ============================================================================
// Cache Invalidation
// ============================================================================

// Invalidate removes a single path from the cache.
func (f *FS) Invalidate(path string) {
	path = normalizePath(path)
	f.log.Debug("metacache.Invalidate", "path", path)

	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()

	delete(f.cache, path)
	delete(f.cache, path+":lstat")
	delete(f.cache, path+":readdir")
}

// InvalidateDir removes a directory and all its children from the cache.
func (f *FS) InvalidateDir(path string) {
	path = normalizePath(path)
	f.log.Debug("metacache.InvalidateDir", "path", path)

	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()

	// Delete the directory itself
	delete(f.cache, path)
	delete(f.cache, path+":lstat")
	delete(f.cache, path+":readdir")

	// Delete all children
	prefix := path + "/"
	if path == "." || path == "" {
		prefix = ""
	}
	for key := range f.cache {
		if prefix != "" && strings.HasPrefix(key, prefix) {
			delete(f.cache, key)
		}
	}
}

// InvalidateAll clears the entire cache.
func (f *FS) InvalidateAll() {
	f.log.Debug("metacache.InvalidateAll")

	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()
	f.cache = make(map[string]*cacheEntry)
}

// ============================================================================
// Internal Cache Helpers
// ============================================================================

// normalizePath ensures consistent path format for cache keys.
func normalizePath(p string) string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return "."
	}
	return p
}

// getCached retrieves a cached entry if it exists and is not expired.
// Returns the entry, whether a refresh-ahead should be triggered, and whether found.
func (f *FS) getCached(key string) (*cacheEntry, bool, bool) {
	f.cacheMu.RLock()
	entry, exists := f.cache[key]
	f.cacheMu.RUnlock()

	if !exists {
		return nil, false, false
	}

	now := time.Now()

	// Expired - remove and return not found
	if now.After(entry.expiresAt) {
		f.cacheMu.Lock()
		delete(f.cache, key)
		f.cacheMu.Unlock()
		f.log.Debug("metacache.expired", "key", key, "expired", time.Since(entry.expiresAt))
		return nil, false, false
	}

	// Check if refresh-ahead should be triggered
	needsRefresh := !entry.renewsAt.IsZero() && now.After(entry.renewsAt)
	if needsRefresh {
		f.log.Debug("metacache.refresh-ahead", "key", key)
	}

	return entry, needsRefresh, true
}

// setCached stores an entry in the cache.
func (f *FS) setCached(key string, info fs.FileInfo, entries []fs.DirEntry, err error) {
	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()

	now := time.Now()
	entry := &cacheEntry{
		info:     info,
		entries:  entries,
		err:      err,
		cachedAt: now,
	}

	if err != nil {
		// Errors get shorter TTL and no refresh-ahead
		entry.expiresAt = now.Add(f.ttl / 2)
		// renewsAt stays zero - no refresh-ahead for errors
	} else {
		entry.renewsAt = now.Add(f.ttl / 2)
		entry.expiresAt = now.Add(f.ttl)
	}

	f.cache[key] = entry
	f.log.Debug("metacache.cached", "key", key, "err", err != nil)
}

// invalidateParent removes the parent directory's readdir cache.
func (f *FS) invalidateParent(path string) {
	path = normalizePath(path)
	parent := filepath.Dir(path)
	if parent == "." || parent == "" {
		parent = "."
	}
	f.cacheMu.Lock()
	delete(f.cache, parent+":readdir")
	f.cacheMu.Unlock()
}

// extendExpiry extends an existing cache entry's expiry on transient refresh errors.
// This prevents entries from expiring when the underlying FS has temporary issues.
func (f *FS) extendExpiry(key string) {
	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()

	entry, exists := f.cache[key]
	if !exists {
		return
	}

	now := time.Now()
	// Extend expiry by another TTL, reset renewsAt
	entry.renewsAt = now.Add(f.ttl / 2)
	entry.expiresAt = now.Add(f.ttl)
	f.log.Debug("metacache.extendExpiry", "key", key)
}

// ============================================================================
// File Wrapper
// ============================================================================

// File wraps an fs.File to track writes and invalidate cache on Close.
type File struct {
	fs.File
	fsys       *FS
	name       string
	written    bool
	dirEntries []fs.DirEntry // cached directory entries for ReadDir
	dirOffset  int           // current offset for ReadDir iteration
}

// wrapFile creates a wrapped file that tracks writes.
func (f *FS) wrapFile(file fs.File, name string) *File {
	return &File{
		File: file,
		fsys: f,
		name: name,
	}
}

// Stat returns cached file info if available, otherwise delegates to underlying file.
func (wf *File) Stat() (fs.FileInfo, error) {
	key := normalizePath(wf.name)

	// Check cache first
	if entry, _, found := wf.fsys.getCached(key); found && entry.err == nil && entry.info != nil {
		return entry.info, nil
	}

	// Delegate to underlying file
	info, err := wf.File.Stat()
	if err == nil {
		// Cache the result
		wf.fsys.setCached(key, info, nil, nil)
	}
	return info, err
}

// Write writes to the file and marks that a write occurred.
func (wf *File) Write(p []byte) (int, error) {
	w, ok := wf.File.(io.Writer)
	if !ok {
		return 0, fs.ErrPermission
	}
	n, err := w.Write(p)
	if n > 0 {
		wf.written = true
	}
	return n, err
}

// WriteAt writes to the file at an offset and marks that a write occurred.
func (wf *File) WriteAt(p []byte, off int64) (int, error) {
	wa, ok := wf.File.(io.WriterAt)
	if !ok {
		return 0, fs.ErrPermission
	}
	n, err := wa.WriteAt(p, off)
	if n > 0 {
		wf.written = true
	}
	return n, err
}

// Close closes the file and invalidates cache if writes were made.
func (wf *File) Close() error {
	err := wf.File.Close()
	if wf.written {
		wf.fsys.Invalidate(wf.name)
		wf.fsys.invalidateParent(wf.name)
	}
	return err
}

// ReadAt delegates to the underlying file.
func (wf *File) ReadAt(p []byte, off int64) (int, error) {
	ra, ok := wf.File.(io.ReaderAt)
	if !ok {
		return fs.ReadAt(wf.File, p, off)
	}
	return ra.ReadAt(p, off)
}

// Seek delegates to the underlying file.
func (wf *File) Seek(offset int64, whence int) (int64, error) {
	s, ok := wf.File.(io.Seeker)
	if !ok {
		return 0, fs.ErrNotSupported
	}
	return s.Seek(offset, whence)
}

// Sync delegates to the underlying file.
func (wf *File) Sync() error {
	if sf, ok := wf.File.(fs.SyncFile); ok {
		return sf.Sync()
	}
	return fs.ErrNotSupported
}

// ReadDir returns directory entries using cached data from the FS.
func (wf *File) ReadDir(n int) ([]fs.DirEntry, error) {
	// Load entries from cache on first call
	if wf.dirEntries == nil {
		entries, err := wf.fsys.ReadDirContext(context.Background(), wf.name)
		if err != nil {
			return nil, err
		}
		wf.dirEntries = entries
		wf.dirOffset = 0
	}

	// Return all remaining entries if n <= 0
	if n <= 0 {
		entries := wf.dirEntries[wf.dirOffset:]
		wf.dirOffset = len(wf.dirEntries)
		return entries, nil
	}

	// Return up to n entries
	remaining := len(wf.dirEntries) - wf.dirOffset
	if remaining == 0 {
		return nil, io.EOF
	}
	if n > remaining {
		n = remaining
	}
	entries := wf.dirEntries[wf.dirOffset : wf.dirOffset+n]
	wf.dirOffset += n
	return entries, nil
}

// ============================================================================
// Open Operations (Return Wrapped Files)
// ============================================================================

// Open opens the named file for reading.
func (f *FS) Open(name string) (fs.File, error) {
	file, err := f.DefaultFS.Open(name)
	if err != nil {
		return nil, err
	}
	return f.wrapFile(file, name), nil
}

// OpenContext opens the named file for reading with context.
func (f *FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	file, err := f.DefaultFS.OpenContext(ctx, name)
	if err != nil {
		return nil, err
	}
	return f.wrapFile(file, name), nil
}

// OpenFile opens a file with the specified flag and permissions.
func (f *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	file, err := f.DefaultFS.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}

	// Invalidate if file might have been created or truncated
	const O_CREATE = 0x40 // syscall.O_CREAT
	const O_TRUNC = 0x200 // syscall.O_TRUNC
	if flag&(O_CREATE|O_TRUNC) != 0 {
		f.Invalidate(name)
		f.invalidateParent(name)
	}

	return f.wrapFile(file, name), nil
}

// Create creates a file and returns a wrapped file handle.
func (f *FS) Create(name string) (fs.File, error) {
	file, err := f.DefaultFS.Create(name)
	if err != nil {
		return nil, err
	}

	f.Invalidate(name)
	f.invalidateParent(name)

	return f.wrapFile(file, name), nil
}

// ============================================================================
// Stat Operations (Cached)
// ============================================================================

// Stat returns cached file info or fetches fresh if expired.
func (f *FS) Stat(name string) (fs.FileInfo, error) {
	return f.StatContext(context.Background(), name)
}

// StatContext returns cached file info or fetches fresh if expired.
func (f *FS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	key := normalizePath(name)

	// Check cache
	if entry, needsRefresh, found := f.getCached(key); found {
		if needsRefresh {
			// Trigger async refresh
			go f.refreshStat(context.Background(), name, key)
		}
		if entry.err != nil {
			return nil, entry.err
		}
		return entry.info, nil
	}

	// Cache miss - fetch synchronously
	return f.fetchAndCacheStat(ctx, name, key)
}

// Lstat returns cached file info (without following symlinks) or fetches fresh.
func (f *FS) Lstat(name string) (fs.FileInfo, error) {
	return f.LstatContext(context.Background(), name)
}

// LstatContext returns cached file info (without following symlinks) or fetches fresh.
func (f *FS) LstatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	key := normalizePath(name) + ":lstat"

	// Check cache
	if entry, needsRefresh, found := f.getCached(key); found {
		if needsRefresh {
			// Trigger async refresh
			go f.refreshLstat(context.Background(), name, key)
		}
		if entry.err != nil {
			return nil, entry.err
		}
		return entry.info, nil
	}

	// Cache miss - fetch synchronously
	return f.fetchAndCacheLstat(ctx, name, key)
}

func (f *FS) fetchAndCacheStat(ctx context.Context, name, key string) (fs.FileInfo, error) {
	info, err := f.DefaultFS.StatContext(ctx, name)
	if err != nil {
		// Only cache ErrNotExist for non-root paths
		// Root directory should always exist; ErrNotExist there is transient
		if errors.Is(err, fs.ErrNotExist) && key != "." {
			f.setCached(key, nil, nil, fs.ErrNotExist)
		}
		return nil, err
	}
	f.setCached(key, info, nil, nil)
	return info, nil
}

func (f *FS) fetchAndCacheLstat(ctx context.Context, name, key string) (fs.FileInfo, error) {
	info, err := f.DefaultFS.LstatContext(ctx, name)
	if err != nil {
		// Only cache ErrNotExist for non-root paths
		if errors.Is(err, fs.ErrNotExist) && key != "." {
			f.setCached(key, nil, nil, fs.ErrNotExist)
		}
		return nil, err
	}
	f.setCached(key, info, nil, nil)
	return info, nil
}

func (f *FS) refreshStat(ctx context.Context, name, key string) {
	info, err := f.DefaultFS.StatContext(ctx, name)
	if err != nil {
		// During refresh-ahead, don't cache errors - just extend existing entry.
		// This prevents transient errors from poisoning the cache.
		// If the file really doesn't exist, the next synchronous fetch will handle it.
		f.extendExpiry(key)
		f.log.Debug("metacache.refreshStat failed", "name", name, "err", err)
		return
	}
	f.setCached(key, info, nil, nil)
}

func (f *FS) refreshLstat(ctx context.Context, name, key string) {
	info, err := f.DefaultFS.LstatContext(ctx, name)
	if err != nil {
		// During refresh-ahead, don't cache errors - just extend existing entry.
		f.extendExpiry(key)
		f.log.Debug("metacache.refreshLstat failed", "name", name, "err", err)
		return
	}
	f.setCached(key, info, nil, nil)
}

// ============================================================================
// ReadDir Operations (Cached)
// ============================================================================

// ReadDirContext returns cached directory entries or fetches fresh if expired.
func (f *FS) ReadDirContext(ctx context.Context, name string) ([]fs.DirEntry, error) {
	key := normalizePath(name) + ":readdir"

	// Check cache
	if entry, needsRefresh, found := f.getCached(key); found {
		if needsRefresh {
			// Trigger async refresh
			go f.refreshReadDir(context.Background(), name, key)
		}
		if entry.err != nil {
			return nil, entry.err
		}
		return entry.entries, nil
	}

	// Cache miss - fetch synchronously
	return f.fetchAndCacheReadDir(ctx, name, key)
}

func (f *FS) fetchAndCacheReadDir(ctx context.Context, name, key string) ([]fs.DirEntry, error) {
	entries, err := f.DefaultFS.ReadDirContext(ctx, name)
	if err != nil {
		// Only cache ErrNotExist for non-root paths
		// Root directory should always exist; ErrNotExist there is transient
		normalizedName := normalizePath(name)
		if errors.Is(err, fs.ErrNotExist) && normalizedName != "." {
			f.setCached(key, nil, nil, fs.ErrNotExist)
		}
		return nil, err
	}
	f.setCached(key, nil, entries, nil)
	return entries, nil
}

func (f *FS) refreshReadDir(ctx context.Context, name, key string) {
	entries, err := f.DefaultFS.ReadDirContext(ctx, name)
	if err != nil {
		// During refresh-ahead, don't cache errors - just extend existing entry.
		// This prevents transient errors from poisoning the cache.
		f.extendExpiry(key)
		f.log.Debug("metacache.refreshReadDir failed", "name", name, "err", err)
		return
	}
	f.setCached(key, nil, entries, nil)
}

// ============================================================================
// Write Operations (Invalidating)
// ============================================================================

// Mkdir creates a directory and invalidates the parent's readdir cache.
func (f *FS) Mkdir(name string, perm fs.FileMode) error {
	err := f.DefaultFS.Mkdir(name, perm)
	if err == nil {
		f.invalidateParent(name)
	}
	return err
}

// MkdirAll creates a directory tree and invalidates affected caches.
func (f *FS) MkdirAll(name string, perm fs.FileMode) error {
	err := f.DefaultFS.MkdirAll(name, perm)
	if err == nil {
		// Invalidate all ancestors that might have been created
		// Walk up the path and invalidate readdir caches
		path := normalizePath(name)
		for path != "." && path != "" {
			f.Invalidate(path)
			f.invalidateParent(path)
			path = filepath.Dir(path)
		}
	}
	return err
}

// Remove removes a file or directory and invalidates caches.
func (f *FS) Remove(name string) error {
	err := f.DefaultFS.Remove(name)
	if err == nil {
		f.Invalidate(name)
		f.invalidateParent(name)
	}
	return err
}

// RemoveAll removes a path and its children, invalidating caches.
func (f *FS) RemoveAll(name string) error {
	err := f.DefaultFS.RemoveAll(name)
	if err == nil {
		f.InvalidateDir(name)
		f.invalidateParent(name)
	}
	return err
}

// Rename renames a file or directory and invalidates affected caches.
func (f *FS) Rename(oldname, newname string) error {
	err := f.DefaultFS.Rename(oldname, newname)
	if err == nil {
		f.Invalidate(oldname)
		f.Invalidate(newname)
		f.invalidateParent(oldname)
		f.invalidateParent(newname)
	}
	return err
}

// Truncate truncates a file and invalidates its cache.
func (f *FS) Truncate(name string, size int64) error {
	err := f.DefaultFS.Truncate(name, size)
	if err == nil {
		f.Invalidate(name)
	}
	return err
}

// Symlink creates a symbolic link and invalidates caches.
func (f *FS) Symlink(oldname, newname string) error {
	err := f.DefaultFS.Symlink(oldname, newname)
	if err == nil {
		f.Invalidate(newname)
		f.invalidateParent(newname)
	}
	return err
}

// Chmod changes file mode and invalidates its cache.
func (f *FS) Chmod(name string, mode fs.FileMode) error {
	err := f.DefaultFS.Chmod(name, mode)
	if err == nil {
		f.Invalidate(name)
	}
	return err
}

// Chown changes file ownership and invalidates its cache.
func (f *FS) Chown(name string, uid, gid int) error {
	err := f.DefaultFS.Chown(name, uid, gid)
	if err == nil {
		f.Invalidate(name)
	}
	return err
}

// Chtimes changes file times and invalidates its cache.
func (f *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	err := f.DefaultFS.Chtimes(name, atime, mtime)
	if err == nil {
		f.Invalidate(name)
	}
	return err
}
