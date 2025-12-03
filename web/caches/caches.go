//go:build js && wasm

// Package caches provides a filesystem that exposes the browser's Cache API.
// Top-level directories represent cache names. Within each cache, URLs are
// represented as a nested directory structure: host/path/to/file.
// Supports reading, writing, and deleting cached entries.
package caches

import (
	"bytes"
	"context"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"syscall/js"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/web/jsutil"
)

// FS exposes the browser's Cache API as a filesystem.
// The root directory contains cache names as subdirectories.
// Each cache contains URLs as nested directories: host/path/to/file
type FS struct{}

// New creates a new caches filesystem.
func New() *FS {
	return &FS{}
}

var _ fs.FS = (*FS)(nil)
var _ fs.ReadDirFS = (*FS)(nil)
var _ fs.RemoveFS = (*FS)(nil)
var _ fs.OpenFileFS = (*FS)(nil)

// caches returns the global caches object
func caches() js.Value {
	return js.Global().Get("caches")
}

// Open opens the named file.
func (fsys *FS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

// OpenContext opens the named file with context.
func (fsys *FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	// Root directory - list all caches
	if name == "." {
		entries, err := fsys.listCaches()
		if err != nil {
			return nil, err
		}
		return fskit.DirFile(fskit.Entry(".", fs.ModeDir|0755), entries...), nil
	}

	// Parse path: first component is cache name
	parts := strings.SplitN(name, "/", 2)
	cacheName := parts[0]

	// Check if cache exists
	exists, err := fsys.cacheExists(cacheName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	// Opening a cache directory
	if len(parts) == 1 {
		entries, err := fsys.listEntriesAt(cacheName, "")
		if err != nil {
			return nil, err
		}
		return fskit.DirFile(fskit.Entry(cacheName, fs.ModeDir|0755), entries...), nil
	}

	// Path within the cache
	subPath := parts[1]
	return fsys.openPath(cacheName, subPath)
}

// OpenFile opens a file with the specified flags.
func (fsys *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrNotExist}
	}

	// Check for write/create flags
	isWrite := flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0

	if !isWrite {
		// Read-only, use regular Open
		return fsys.Open(name)
	}

	// Writing requires at least cache/host/path structure
	parts := strings.SplitN(name, "/", 2)
	if len(parts) < 2 {
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrInvalid}
	}

	cacheName := parts[0]
	subPath := parts[1]

	// Validate the path has at least host/file structure
	if !strings.Contains(subPath, "/") {
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrInvalid}
	}

	// Find existing URL or determine the URL for new file
	existingURL, _ := fsys.findURLForPath(cacheName, subPath)
	requestURL := existingURL
	if requestURL == "" {
		requestURL = fsys.pathToURL(subPath)
	}

	// Check if file exists (for O_EXCL)
	if flag&os.O_EXCL != 0 && existingURL != "" {
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: fs.ErrExist}
	}

	// Create a writable file
	var existingData []byte

	// If not truncating, try to load existing data
	if flag&os.O_TRUNC == 0 && existingURL != "" {
		cache, err := jsutil.AwaitErr(caches().Call("open", cacheName))
		if err == nil {
			response, err := jsutil.AwaitErr(cache.Call("match", existingURL))
			if err == nil && !response.IsUndefined() && !response.IsNull() {
				arrayBuffer, err := jsutil.AwaitErr(response.Call("arrayBuffer"))
				if err == nil {
					uint8Array := js.Global().Get("Uint8Array").New(arrayBuffer)
					existingData = make([]byte, uint8Array.Length())
					js.CopyBytesToGo(existingData, uint8Array)
				}
			}
		}
	}

	// Create the cache entry immediately so stat works after open
	// This ensures the file "exists" even before Close is called
	cache, err := jsutil.AwaitErr(caches().Call("open", cacheName))
	if err != nil {
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: err}
	}

	// Create an initial response with the existing or empty data
	uint8Array := js.Global().Get("Uint8Array").New(len(existingData))
	if len(existingData) > 0 {
		js.CopyBytesToJS(uint8Array, existingData)
	}
	response := js.Global().Get("Response").New(uint8Array, map[string]any{
		"status":     200,
		"statusText": "OK",
		"headers": map[string]any{
			"Content-Length": len(existingData),
		},
	})
	_, err = jsutil.AwaitErr(cache.Call("put", requestURL, response))
	if err != nil {
		return nil, &fs.PathError{Op: "openfile", Path: name, Err: err}
	}

	return &writableFile{
		fsys:       fsys,
		cacheName:  cacheName,
		subPath:    subPath,
		name:       path.Base(subPath),
		requestURL: requestURL,
		data:       existingData,
		dirty:      false,
		append:     flag&os.O_APPEND != 0,
	}, nil
}

// ReadDir reads the named directory.
func (fsys *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}

	if name == "." {
		return fsys.listCaches()
	}

	// Parse path
	parts := strings.SplitN(name, "/", 2)
	cacheName := parts[0]

	exists, err := fsys.cacheExists(cacheName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}

	subPath := ""
	if len(parts) > 1 {
		subPath = parts[1]
	}

	entries, err := fsys.listEntriesAt(cacheName, subPath)
	if err != nil {
		return nil, err
	}
	// Return empty slice for empty directories (not an error)
	if entries == nil {
		return []fs.DirEntry{}, nil
	}
	return entries, nil
}

// Remove removes a cached file or an entire cache.
func (fsys *FS) Remove(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	if name == "." {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
	}

	parts := strings.SplitN(name, "/", 2)
	cacheName := parts[0]

	// Delete entire cache
	if len(parts) == 1 {
		deleted, err := jsutil.AwaitErr(caches().Call("delete", cacheName))
		if err != nil {
			return &fs.PathError{Op: "remove", Path: name, Err: err}
		}
		if !deleted.Bool() {
			return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
		}
		return nil
	}

	// Delete a specific entry from a cache
	subPath := parts[1]

	// Find the matching URL
	matchedURL, err := fsys.findURLForPath(cacheName, subPath)
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}
	if matchedURL == "" {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	cache, err := jsutil.AwaitErr(caches().Call("open", cacheName))
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}

	deleted, err := jsutil.AwaitErr(cache.Call("delete", matchedURL))
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}
	if !deleted.Bool() {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}
	return nil
}

// Stat returns file info for the named file.
func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	return fsys.StatContext(context.Background(), name)
}

// StatContext returns file info for the named file with context.
func (fsys *FS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}

	// Root directory
	if name == "." {
		return fskit.Entry(".", fs.ModeDir|0755), nil
	}

	parts := strings.SplitN(name, "/", 2)
	cacheName := parts[0]

	exists, err := fsys.cacheExists(cacheName)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}
	if !exists {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}

	// Cache directory
	if len(parts) == 1 {
		return fskit.Entry(cacheName, fs.ModeDir|0755), nil
	}

	// Path within cache
	subPath := parts[1]
	return fsys.statPath(cacheName, subPath)
}

// listCaches returns all cache names as directory entries.
func (fsys *FS) listCaches() ([]fs.DirEntry, error) {
	keys, err := jsutil.AwaitErr(caches().Call("keys"))
	if err != nil {
		return nil, err
	}

	var entries []fs.DirEntry
	length := keys.Length()
	for i := 0; i < length; i++ {
		name := keys.Index(i).String()
		entries = append(entries, fskit.Entry(name, fs.ModeDir|0755))
	}
	return entries, nil
}

// cacheExists checks if a cache with the given name exists.
func (fsys *FS) cacheExists(name string) (bool, error) {
	has, err := jsutil.AwaitErr(caches().Call("has", name))
	if err != nil {
		return false, err
	}
	return has.Bool(), nil
}

// getAllCacheURLs returns all URLs in a cache.
func (fsys *FS) getAllCacheURLs(cacheName string) ([]string, error) {
	cache, err := jsutil.AwaitErr(caches().Call("open", cacheName))
	if err != nil {
		return nil, err
	}

	keys, err := jsutil.AwaitErr(cache.Call("keys"))
	if err != nil {
		return nil, err
	}

	var urls []string
	length := keys.Length()
	for i := 0; i < length; i++ {
		request := keys.Index(i)
		requestURL := request.Get("url").String()
		urls = append(urls, requestURL)
	}
	return urls, nil
}

// urlToPath converts a URL to a filesystem path (without scheme).
// Example: "http://localhost:8788/bundles/file.tar" -> "localhost:8788/bundles/file.tar"
func (fsys *FS) urlToPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// Build path: host/path (no scheme)
	p := u.Host + u.Path
	if u.RawQuery != "" {
		// Encode query string to be filesystem-safe
		p += "?" + u.RawQuery
	}
	return p
}

// pathToURL converts a filesystem path to a URL.
// It infers the scheme: http for localhost/127.0.0.1, https otherwise.
// Example: "localhost:8788/bundles/file.tar" -> "http://localhost:8788/bundles/file.tar"
func (fsys *FS) pathToURL(filePath string) string {
	// Extract host from path
	parts := strings.SplitN(filePath, "/", 2)
	host := parts[0]

	// Infer scheme
	scheme := "https"
	hostWithoutPort := strings.Split(host, ":")[0]
	if hostWithoutPort == "localhost" || hostWithoutPort == "127.0.0.1" || hostWithoutPort == "0.0.0.0" {
		scheme = "http"
	}

	return scheme + "://" + filePath
}

// findURLForPath finds the full URL that matches the given path (without scheme).
// It searches through all cache entries to find a match.
func (fsys *FS) findURLForPath(cacheName, filePath string) (string, error) {
	urls, err := fsys.getAllCacheURLs(cacheName)
	if err != nil {
		return "", err
	}

	for _, rawURL := range urls {
		if fsys.urlToPath(rawURL) == filePath {
			return rawURL, nil
		}
	}
	return "", nil
}

// listEntriesAt lists directory entries at a given path within a cache.
// Returns nil if the path doesn't exist as a directory.
func (fsys *FS) listEntriesAt(cacheName, dirPath string) ([]fs.DirEntry, error) {
	urls, err := fsys.getAllCacheURLs(cacheName)
	if err != nil {
		return nil, err
	}

	// Convert all URLs to paths and find entries at the given level
	entrySet := make(map[string]bool) // track unique entries (true = dir, false = file)

	prefix := dirPath
	if prefix != "" {
		prefix += "/"
	}

	foundPrefix := false
	for _, rawURL := range urls {
		urlPath := fsys.urlToPath(rawURL)
		if urlPath == "" {
			continue
		}

		// Check if this URL is under our directory
		if dirPath == "" {
			// At cache root, get first path component (host)
			parts := strings.SplitN(urlPath, "/", 2)
			entrySet[parts[0]] = true // host is always a directory
			foundPrefix = true
		} else if strings.HasPrefix(urlPath, prefix) {
			// Under our directory
			remainder := strings.TrimPrefix(urlPath, prefix)
			parts := strings.SplitN(remainder, "/", 2)
			name := parts[0]
			isDir := len(parts) > 1
			// If we already marked it as a dir, keep it as dir
			if existing, ok := entrySet[name]; ok && existing {
				isDir = true
			}
			entrySet[name] = isDir
			foundPrefix = true
		} else if urlPath == dirPath {
			// Exact match - this is a file, not a directory
			return nil, nil
		}
	}

	if !foundPrefix && dirPath != "" {
		return nil, nil // directory doesn't exist
	}

	// Convert to sorted entries
	var names []string
	for name := range entrySet {
		names = append(names, name)
	}
	sort.Strings(names)

	var entries []fs.DirEntry
	for _, name := range names {
		isDir := entrySet[name]
		mode := fs.FileMode(0644)
		if isDir {
			mode = fs.ModeDir | 0755
		}
		entries = append(entries, fskit.Entry(name, mode))
	}

	return entries, nil
}

// openPath opens a file or directory at the given path within a cache.
func (fsys *FS) openPath(cacheName, subPath string) (fs.File, error) {
	// First check if this is an exact file match
	matchedURL, err := fsys.findURLForPath(cacheName, subPath)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path.Join(cacheName, subPath), Err: err}
	}

	if matchedURL != "" {
		cache, err := jsutil.AwaitErr(caches().Call("open", cacheName))
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: path.Join(cacheName, subPath), Err: err}
		}

		response, err := jsutil.AwaitErr(cache.Call("match", matchedURL))
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: path.Join(cacheName, subPath), Err: err}
		}

		if !response.IsUndefined() && !response.IsNull() {
			// It's a file - read and return it
			return fsys.responseToFile(response, path.Base(subPath))
		}
	}

	// Check if it's a directory with entries
	entries, err := fsys.listEntriesAt(cacheName, subPath)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path.Join(cacheName, subPath), Err: err}
	}
	if entries != nil {
		return fskit.DirFile(fskit.Entry(path.Base(subPath), fs.ModeDir|0755), entries...), nil
	}

	// For caches filesystem, treat host-only paths as virtual directories
	// This allows opening host directories when they're empty
	if !strings.Contains(subPath, "/") {
		// Just a host, treat as empty directory
		return fskit.DirFile(fskit.Entry(path.Base(subPath), fs.ModeDir|0755)), nil
	}

	return nil, &fs.PathError{Op: "open", Path: path.Join(cacheName, subPath), Err: fs.ErrNotExist}
}

// statPath returns file info for a path within a cache.
func (fsys *FS) statPath(cacheName, subPath string) (fs.FileInfo, error) {
	// First check if this is an exact file match
	matchedURL, err := fsys.findURLForPath(cacheName, subPath)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: path.Join(cacheName, subPath), Err: err}
	}

	if matchedURL != "" {
		cache, err := jsutil.AwaitErr(caches().Call("open", cacheName))
		if err != nil {
			return nil, &fs.PathError{Op: "stat", Path: path.Join(cacheName, subPath), Err: err}
		}

		response, err := jsutil.AwaitErr(cache.Call("match", matchedURL))
		if err != nil {
			return nil, &fs.PathError{Op: "stat", Path: path.Join(cacheName, subPath), Err: err}
		}

		if !response.IsUndefined() && !response.IsNull() {
			// It's a file
			var size int64
			contentLength := response.Get("headers").Call("get", "content-length")
			if !contentLength.IsNull() && !contentLength.IsUndefined() {
				if cl := contentLength.String(); cl != "" {
					var n int64
					for _, c := range cl {
						if c >= '0' && c <= '9' {
							n = n*10 + int64(c-'0')
						}
					}
					size = n
				}
			}
			return &cachedFileInfo{
				name: path.Base(subPath),
				size: size,
			}, nil
		}
	}

	// Check if it's a directory with entries
	entries, err := fsys.listEntriesAt(cacheName, subPath)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: path.Join(cacheName, subPath), Err: err}
	}
	if entries != nil {
		return fskit.Entry(path.Base(subPath), fs.ModeDir|0755), nil
	}

	// For caches filesystem, treat host-only paths as virtual directories
	// This allows creating files when the host directory is empty
	// A host-only path is like "localhost:8788" (no slash after the host)
	if !strings.Contains(subPath, "/") {
		// Just a host, treat as directory
		return fskit.Entry(path.Base(subPath), fs.ModeDir|0755), nil
	}

	return nil, &fs.PathError{Op: "stat", Path: path.Join(cacheName, subPath), Err: fs.ErrNotExist}
}

// responseToFile converts a cache Response to a file.
func (fsys *FS) responseToFile(response js.Value, name string) (fs.File, error) {
	// Get the response body as ArrayBuffer
	arrayBuffer, err := jsutil.AwaitErr(response.Call("arrayBuffer"))
	if err != nil {
		return nil, err
	}

	// Convert ArrayBuffer to bytes
	uint8Array := js.Global().Get("Uint8Array").New(arrayBuffer)
	data := make([]byte, uint8Array.Length())
	js.CopyBytesToGo(data, uint8Array)

	return &cachedFile{
		name:   name,
		data:   data,
		size:   int64(len(data)),
		reader: bytes.NewReader(data),
	}, nil
}

// cachedFile implements fs.File for a cached response (read-only).
type cachedFile struct {
	name   string
	data   []byte
	size   int64
	reader *bytes.Reader
}

var _ fs.File = (*cachedFile)(nil)

func (f *cachedFile) Stat() (fs.FileInfo, error) {
	return &cachedFileInfo{
		name: f.name,
		size: f.size,
	}, nil
}

func (f *cachedFile) Read(p []byte) (int, error) {
	return f.reader.Read(p)
}

func (f *cachedFile) Close() error {
	return nil
}

func (f *cachedFile) Seek(offset int64, whence int) (int64, error) {
	return f.reader.Seek(offset, whence)
}

// writableFile implements fs.File for writing to the cache.
type writableFile struct {
	fsys       *FS
	cacheName  string
	subPath    string
	name       string
	requestURL string
	data       []byte
	offset     int64
	dirty      bool
	append     bool
}

var _ fs.File = (*writableFile)(nil)

func (f *writableFile) Stat() (fs.FileInfo, error) {
	return &cachedFileInfo{
		name: f.name,
		size: int64(len(f.data)),
	}, nil
}

func (f *writableFile) Read(p []byte) (int, error) {
	if f.offset >= int64(len(f.data)) {
		return 0, fs.ErrClosed
	}
	n := copy(p, f.data[f.offset:])
	f.offset += int64(n)
	return n, nil
}

func (f *writableFile) Write(p []byte) (int, error) {
	if f.append {
		f.data = append(f.data, p...)
	} else {
		// Grow data slice if necessary
		end := f.offset + int64(len(p))
		if end > int64(len(f.data)) {
			newData := make([]byte, end)
			copy(newData, f.data)
			f.data = newData
		}
		copy(f.data[f.offset:], p)
		f.offset = end
	}
	f.dirty = true
	return len(p), nil
}

func (f *writableFile) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case 0: // io.SeekStart
		newOffset = offset
	case 1: // io.SeekCurrent
		newOffset = f.offset + offset
	case 2: // io.SeekEnd
		newOffset = int64(len(f.data)) + offset
	}
	if newOffset < 0 {
		return 0, fs.ErrInvalid
	}
	f.offset = newOffset
	return newOffset, nil
}

func (f *writableFile) Close() error {
	if !f.dirty {
		return nil
	}

	// Open the cache
	cache, err := jsutil.AwaitErr(caches().Call("open", f.cacheName))
	if err != nil {
		return err
	}

	// Create a Uint8Array from the data
	uint8Array := js.Global().Get("Uint8Array").New(len(f.data))
	js.CopyBytesToJS(uint8Array, f.data)

	// Create a Response with the data
	response := js.Global().Get("Response").New(uint8Array, map[string]any{
		"status":     200,
		"statusText": "OK",
		"headers": map[string]any{
			"Content-Length": len(f.data),
		},
	})

	// Store in cache using the URL we determined at open time
	_, err = jsutil.AwaitErr(cache.Call("put", f.requestURL, response))
	return err
}

// cachedFileInfo implements fs.FileInfo for a cached file.
type cachedFileInfo struct {
	name string
	size int64
}

var _ fs.FileInfo = (*cachedFileInfo)(nil)

func (fi *cachedFileInfo) Name() string       { return fi.name }
func (fi *cachedFileInfo) Size() int64        { return fi.size }
func (fi *cachedFileInfo) Mode() fs.FileMode  { return 0644 }
func (fi *cachedFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *cachedFileInfo) IsDir() bool        { return false }
func (fi *cachedFileInfo) Sys() any           { return nil }
