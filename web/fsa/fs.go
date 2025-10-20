//go:build js && wasm

package fsa

import (
	"context"
	"log"
	"path"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/web/jsutil"
)

// statCacheEntry represents a cached stat result (similar to httpfs headCacheEntry)
type statCacheEntry struct {
	info      fs.FileInfo
	err       error // cached errors (e.g., fs.ErrNotExist)
	cachedAt  time.Time
	expiresAt time.Time
}

// FS implements a filesystem interface for the browser's File System Access API.
// It provides access to both user-selected directories and the Origin Private File System (OPFS).
type FS struct {
	handle    js.Value
	statCache map[string]*statCacheEntry // Similar to httpfs headCache
	cacheTTL  time.Duration              // Similar to httpfs cacheTTL
	cacheMu   sync.RWMutex               // Similar to httpfs cacheMu
	opfsRoot  *FS                        // Reference to OPFS root
}

// NewFS creates a new FS instance from a JavaScript directory handle
func NewFS(handle js.Value) *FS {
	return &FS{
		handle:    handle,
		statCache: make(map[string]*statCacheEntry),
		cacheTTL:  500 * time.Millisecond, // Similar to httpfs default
	}
}

// getCachedStat retrieves cached stat result if valid (similar to httpfs getCachedHead)
func (fsys *FS) getCachedStat(path string) (fs.FileInfo, error, bool) {
	fsys.cacheMu.RLock()
	defer fsys.cacheMu.RUnlock()

	entry, exists := fsys.statCache[path]
	if !exists {
		return nil, nil, false
	}

	if time.Now().After(entry.expiresAt) {
		// Entry expired, but don't clean it up here to avoid lock upgrade
		return nil, nil, false
	}

	// If this is a cached error, return it
	if entry.err != nil {
		return nil, entry.err, true
	}

	return entry.info, nil, true
}

// setCachedStat stores stat result in cache (similar to httpfs setCachedHead)
func (fsys *FS) setCachedStat(path string, info fs.FileInfo) {
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()

	now := time.Now()
	fsys.statCache[path] = &statCacheEntry{
		info:      info,
		err:       nil,
		cachedAt:  now,
		expiresAt: now.Add(fsys.cacheTTL),
	}
}

// setCachedStatError stores stat error in cache (similar to httpfs setCachedHeadError)
func (fsys *FS) setCachedStatError(path string, err error) {
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()

	now := time.Now()
	fsys.statCache[path] = &statCacheEntry{
		info:      nil,
		err:       err,
		cachedAt:  now,
		expiresAt: now.Add(fsys.cacheTTL),
	}
}

// invalidateCachedStat removes cached stat result (similar to httpfs invalidateCachedHead)
func (fsys *FS) invalidateCachedStat(path string) {
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()
	delete(fsys.statCache, path)
}

// SetCacheTTL sets the cache TTL for stat requests (similar to httpfs)
func (fsys *FS) SetCacheTTL(ttl time.Duration) {
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()
	fsys.cacheTTL = ttl
}

// buildFileInfo constructs fs.FileInfo from JS API + metadata store
func (fsys *FS) buildFileInfo(path string, jsHandle js.Value) (fs.FileInfo, error) {
	// Get JS API data (always fresh)
	var name string
	var size int64
	var isDir bool
	var jsMtime time.Time

	if jsHandle.Get("kind").String() == "directory" {
		isDir = true
		name = jsHandle.Get("name").String()
		size = 0
		// Directories don't have lastModified in JS API
		jsMtime = time.Now()
	} else {
		// Get file data
		file, err := jsutil.AwaitErr(jsHandle.Call("getFile"))
		if err != nil {
			return nil, err
		}
		name = jsHandle.Get("name").String()
		size = int64(file.Get("size").Int())
		jsMtime = time.UnixMilli(int64(file.Get("lastModified").Int()))
	}

	// Get metadata from global store
	metadata, hasMetadata := GetMetadataStore().GetMetadata(path)

	var mode fs.FileMode
	var mtime, atime time.Time

	if hasMetadata {
		// Use stored metadata
		mode = metadata.Mode
		mtime = metadata.Mtime // We manage mtime, ignore JS mtime
		atime = metadata.Atime
	} else {
		// Use defaults for new files
		if isDir {
			mode = DefaultDirMode | fs.ModeDir
		} else {
			mode = DefaultFileMode
		}
		mtime = jsMtime // Use JS mtime for new files
		atime = time.Now()

		// Store initial metadata
		GetMetadataStore().SetMetadata(path, FileMetadata{
			Mode:  mode,
			Mtime: mtime,
			Atime: atime,
		})
	}

	// Ensure directory bit is set correctly
	if isDir {
		mode |= fs.ModeDir
	}

	// log.Println("buildFileInfo", path, hasMetadata, mode, isDir, mode&fs.ModeDir != 0)

	return fskit.Entry(name, mode, size, mtime), nil
}

func (fsys *FS) walkDir(path string) (js.Value, error) {
	if path == "." {
		return fsys.handle, nil
	}
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	cur := fsys.handle
	var err error
	for i := 0; i < len(parts); i++ {
		cur, err = jsutil.AwaitErr(cur.Call("getDirectoryHandle", parts[i], map[string]any{"create": false}))
		if err != nil {
			return js.Undefined(), &fs.PathError{Op: "walkdir", Path: path, Err: err}
		}
	}
	return cur, nil
}

func (fsys *FS) Symlink(oldname, newname string) error {
	if !fs.ValidPath(newname) {
		return &fs.PathError{Op: "symlink", Path: newname, Err: fs.ErrInvalid}
	}

	err := fs.WriteFile(fsys, newname, []byte(oldname), 0777)
	if err != nil {
		return &fs.PathError{Op: "symlink", Path: newname, Err: err}
	}

	// Update metadata store with symlink mode
	GetMetadataStore().SetMode(newname, fs.FileMode(0777)|fs.ModeSymlink)

	// Invalidate stat cache
	fsys.invalidateCachedStat(newname)

	return nil
}

func (fsys *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chtimes", Path: name, Err: fs.ErrInvalid}
	}

	// Update metadata store
	GetMetadataStore().SetTimes(name, atime, mtime)

	// Invalidate stat cache
	fsys.invalidateCachedStat(name)

	return nil
}

func (fsys *FS) Chmod(name string, mode fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chmod", Path: name, Err: fs.ErrInvalid}
	}

	// Update metadata store
	GetMetadataStore().SetMode(name, mode)

	// Invalidate stat cache
	fsys.invalidateCachedStat(name)

	return nil
}

func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	return fsys.StatContext(context.Background(), name)
}

func (fsys *FS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	// Check cache first
	if info, err, found := fsys.getCachedStat(name); found {
		if err != nil {
			return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
		}
		return info, nil
	}

	// Open file to get JS handle, then build stat info
	f, err := fsys.OpenContext(fs.WithNoFollow(ctx), name)
	if err != nil {
		// Cache the error
		fsys.setCachedStatError(name, err)
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}
	defer f.Close()

	// Get the JS handle from the file
	var jsHandle js.Value
	if fileHandle, ok := f.(*FileHandle); ok {
		jsHandle = fileHandle.Value
	} else {
		// Fallback to regular Stat for directories or other file types
		return f.Stat()
	}

	// Build file info from JS API + metadata store
	info, err := fsys.buildFileInfo(name, jsHandle)
	if err != nil {
		fsys.setCachedStatError(name, err)
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}

	// Cache the result
	fsys.setCachedStat(name, info)
	return info, nil
}

func (fsys *FS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

func (fsys *FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	if name == "." {
		return fsys.openDirectory(".", fsys.handle), nil
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return nil, fs.ErrNotExist
		//return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	file, err := jsutil.AwaitErr(dirHandle.Call("getFileHandle", path.Base(name), map[string]any{"create": false}))
	if err == nil {
		if cached, _, ok := fsys.getCachedStat(name); ok && fs.FollowSymlinks(ctx) && fs.IsSymlink(cached.Mode()) {
			if origin, fullname, ok := fs.Origin(ctx); ok {
				target, err := fs.Readlink(fsys, name)
				if err != nil {
					return nil, &fs.PathError{Op: "open", Path: name, Err: err}
				}
				if strings.HasPrefix(target, "/") {
					target = target[1:]
				} else {
					target = path.Join(strings.TrimSuffix(fullname, name), target)
				}
				return fs.OpenContext(ctx, origin, target)
			} else {
				log.Println("fsa: opencontext: no origin for symlink:", name)
				return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
			}
		}
		return fsys.openFile(name, file, true), nil
	}

	dir, err := jsutil.AwaitErr(dirHandle.Call("getDirectoryHandle", path.Base(name), map[string]any{"create": false}))
	if err == nil {
		return fsys.openDirectory(name, dir), nil
	}

	return nil, fs.ErrNotExist
	//return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (fsys *FS) Truncate(name string, size int64) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "truncate", Path: name, Err: fs.ErrInvalid}
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return &fs.PathError{Op: "truncate", Path: name, Err: fs.ErrNotExist}
	}

	file, err := jsutil.AwaitErr(dirHandle.Call("getFileHandle", path.Base(name), map[string]any{"create": false}))
	if err == nil {
		handle := fsys.openFile(name, file, false)
		err := handle.Truncate(size)
		if err != nil {
			return &fs.PathError{Op: "truncate", Path: name, Err: err}
		}
		err = handle.Close()
		if err != nil {
			return &fs.PathError{Op: "truncate", Path: name, Err: err}
		}
		return err
	}

	return &fs.PathError{Op: "truncate", Path: name, Err: fs.ErrNotExist}
}

func (fsys *FS) Create(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrInvalid}
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrNotExist}
	}

	handle, err := jsutil.AwaitErr(dirHandle.Call("getFileHandle", path.Base(name), map[string]any{"create": true}))
	if err != nil {
		return nil, &fs.PathError{Op: "create", Path: name, Err: err}
	}

	// Invalidate stat cache since we created/truncated a file
	fsys.invalidateCachedStat(name)

	return fsys.openFile(name, handle, false), nil
}

func (fsys *FS) Mkdir(name string, perm fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrInvalid}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: err}
	}
	if ok {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrExist}
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
	}

	_, err = jsutil.AwaitErr(dirHandle.Call("getDirectoryHandle", path.Base(name), map[string]any{"create": true}))
	if err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: err}
	}

	// todo: set perms

	// Invalidate stat cache since we created a directory
	fsys.invalidateCachedStat(name)

	return nil
}

func (fsys *FS) Remove(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}
	if !ok {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	if isDir, err := fs.IsDir(fsys, name); err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	} else if isDir {
		empty, err := fs.IsEmpty(fsys, name)
		if err != nil {
			return &fs.PathError{Op: "remove", Path: name, Err: err}
		}
		if !empty {
			return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotEmpty}
		}
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	_, err = jsutil.AwaitErr(dirHandle.Call("removeEntry", path.Base(name)))
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}

	// Clean up metadata and cache
	GetMetadataStore().DeleteMetadata(name)
	fsys.invalidateCachedStat(name)

	return nil
}

func (fsys *FS) Rename(oldname, newname string) error {
	if !fs.ValidPath(oldname) || !fs.ValidPath(newname) {
		return &fs.PathError{Op: "rename", Path: oldname, Err: fs.ErrInvalid}
	}

	ok, err := fs.Exists(fsys, oldname)
	if err != nil {
		return &fs.PathError{Op: "rename", Path: oldname, Err: err}
	}
	if !ok {
		return &fs.PathError{Op: "rename", Path: oldname, Err: fs.ErrNotExist}
	}

	ok, err = fs.Exists(fsys, newname)
	if err != nil {
		return &fs.PathError{Op: "rename", Path: newname, Err: err}
	}
	if ok {
		return &fs.PathError{Op: "rename", Path: newname, Err: fs.ErrExist}
	}

	oldDirHandle, err := fsys.walkDir(path.Dir(oldname))
	if err != nil {
		return &fs.PathError{Op: "rename", Path: oldname, Err: fs.ErrNotExist}
	}

	newDirHandle, err := fsys.walkDir(path.Dir(newname))
	if err != nil {
		return &fs.PathError{Op: "rename", Path: newname, Err: fs.ErrNotExist}
	}

	if err := fs.CopyAll(fsys, oldname, newname); err != nil {
		return &fs.PathError{Op: "rename", Path: oldname, Err: err}
	}

	_, err = jsutil.AwaitErr(oldDirHandle.Call("removeEntry", path.Base(oldname), map[string]any{"recursive": true}))
	if err != nil {
		// Try to clean up the copy if delete fails
		newDirHandle.Call("removeEntry", path.Base(newname), map[string]any{"recursive": true})
		return &fs.PathError{Op: "rename", Path: oldname, Err: err}
	}

	// Handle metadata for rename: copy metadata from old to new path, then delete old
	if metadata, exists := GetMetadataStore().GetMetadata(oldname); exists {
		GetMetadataStore().SetMetadata(newname, metadata)
	}
	GetMetadataStore().DeleteMetadata(oldname)

	// Invalidate both paths in cache
	fsys.invalidateCachedStat(oldname)
	fsys.invalidateCachedStat(newname)

	return nil
}

// openDirectory creates a directory file handle with lazy loading
func (fsys *FS) openDirectory(dirPath string, handle js.Value) fs.File {
	// log.Println("fsa: opendirectory:", dirPath)
	var entries []fs.DirEntry
	err := jsutil.AsyncIter(handle.Call("values"), func(e js.Value) error {
		entryName := e.Get("name").String()
		isDir := e.Get("kind").String() == "directory"

		// Construct full path for cache lookup
		var entryPath string
		if dirPath == "." {
			entryPath = entryName
		} else {
			entryPath = dirPath + "/" + entryName
		}

		var mode fs.FileMode
		var size int64

		// Get metadata from global store
		if metadata, hasMetadata := GetMetadataStore().GetMetadata(entryPath); hasMetadata {
			mode = metadata.Mode
			size = 0 // Size will be loaded lazily when needed
		} else {
			// Set default modes for new entries
			if isDir {
				mode = DefaultDirMode | fs.ModeDir
				size = 0
			} else {
				mode = DefaultFileMode
				size = 0
			}
		}

		if isDir {
			mode |= fs.ModeDir
		}

		entries = append(entries, fskit.Entry(entryName, mode, size))
		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	return fskit.DirFile(fskit.Entry(dirPath, 0755|fs.ModeDir), entries...)
}

// openFile creates a file handle for the given JavaScript file handle
func (fsys *FS) openFile(path string, handle js.Value, append bool) *FileHandle {
	return &FileHandle{Value: handle, path: path, append: append, fsys: fsys}
}
