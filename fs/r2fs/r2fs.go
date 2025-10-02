package r2fs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"tractor.dev/wanix/fs"
)

// TODO:
// - xattrs
// - revisit ownership

// headCacheEntry represents a cached HEAD request result
type headCacheEntry struct {
	info      *r2FileInfo
	err       error // stores cached errors (e.g., fs.ErrNotExist for 404)
	cachedAt  time.Time
	expiresAt time.Time
}

// FS implements an R2-backed filesystem following the R2 design specification
type FS struct {
	client     *s3.Client
	bucketName string
	basePath   string
	headCache  map[string]*headCacheEntry
	cacheTTL   time.Duration
	cacheMu    sync.RWMutex
}

// New creates a new R2 filesystem with the given credentials and bucket
func New(accountID, accessKeyID, accessKeySecret, bucketName string) (*FS, error) {
	return NewWithBasePath(accountID, accessKeyID, accessKeySecret, bucketName, "")
}

// NewWithBasePath creates a new R2 filesystem with the given credentials, bucket, and base path
func NewWithBasePath(accountID, accessKeyID, accessKeySecret, bucketName, basePath string) (*FS, error) {
	client, err := setupR2Client(accountID, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, err
	}

	return &FS{
		client:     client,
		bucketName: bucketName,
		basePath:   basePath,
		headCache:  make(map[string]*headCacheEntry),
		cacheTTL:   500 * time.Millisecond,
	}, nil
}

// NewWithClient creates a new R2 filesystem with a pre-configured S3 client and bucket
func NewWithClient(client *s3.Client, bucketName string) *FS {
	return NewWithClientAndBasePath(client, bucketName, "")
}

// NewWithClientAndBasePath creates a new R2 filesystem with a pre-configured S3 client, bucket, and base path
func NewWithClientAndBasePath(client *s3.Client, bucketName, basePath string) *FS {
	return &FS{
		client:     client,
		bucketName: bucketName,
		basePath:   basePath,
		headCache:  make(map[string]*headCacheEntry),
		cacheTTL:   500 * time.Millisecond,
	}
}

// setupR2Client creates an S3 client configured for Cloudflare R2
func setupR2Client(accountID, accessKeyID, accessKeySecret string) (*s3.Client, error) {
	// R2 endpoint URL
	r2Endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	// Create custom endpoint resolver
	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           r2Endpoint,
				SigningRegion: "auto",
			}, nil
		})

	// Load the configuration with R2-specific settings
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, accessKeySecret, "")),
		config.WithRegion("auto"),
	)

	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	// Create S3 client with R2 configuration
	client := s3.NewFromConfig(cfg)

	return client, nil
}

// r2File represents an R2-backed file
type r2File struct {
	fs       *FS
	path     string
	isDir    bool
	isDirty  bool
	updated  time.Time
	fileInfo *r2FileInfo
	content  []byte
	pos      int64
	closed   bool
	entries  []fs.DirEntry
	dirPos   int
}

// r2FileInfo holds file metadata
type r2FileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *r2FileInfo) Name() string       { return fi.name }
func (fi *r2FileInfo) Size() int64        { return fi.size }
func (fi *r2FileInfo) Mode() fs.FileMode  { return fi.mode }
func (fi *r2FileInfo) ModTime() time.Time { return fi.modTime }
func (fi *r2FileInfo) IsDir() bool        { return fi.isDir }
func (fi *r2FileInfo) Sys() interface{}   { return nil }

// normalizeR2Path ensures proper R2 object key formatting and prepends basePath
func (fsys *FS) normalizeR2Path(name string) string {
	// Handle root case: "." becomes "/"
	isRoot := (name == "." || name == "")
	if isRoot {
		name = "/"
	} else {
		// Ensure path starts with / for R2
		if !strings.HasPrefix(name, "/") {
			name = "/" + name
		}

		// Clean the path
		name = filepath.Clean(name)

		// Convert backslashes to forward slashes for R2
		name = strings.ReplaceAll(name, "\\", "/")

		// Ensure root stays as "/"
		if name == "" {
			name = "/"
			isRoot = true
		}
	}

	var result string

	// Prepend basePath if it's set and not empty
	if fsys.basePath != "" {
		// Normalize basePath - should NOT start with / (it's a prefix, not absolute path)
		basePath := strings.TrimPrefix(fsys.basePath, "/")
		basePath = filepath.Clean(basePath)
		basePath = strings.ReplaceAll(basePath, "\\", "/")
		basePath = strings.TrimSuffix(basePath, "/")

		// Ensure basePath is not empty after normalization
		if basePath == "" || basePath == "." {
			result = name
		} else {
			// When we have a basePath, the "root" within that basePath should not have trailing slash
			// Only the true filesystem root (no basePath) gets the trailing slash
			if isRoot {
				result = "/" + basePath
			} else {
				result = "/" + basePath + name
			}
		}
	} else {
		result = name
	}

	return result
}

// parseFileMode converts a Unix mode string to fs.FileMode
func parseFileMode(modeStr string) fs.FileMode {
	if modeStr == "" {
		return 0
	}
	unixMode, err := strconv.ParseUint(modeStr, 10, 32)
	if err != nil {
		return 0
	}
	return unixModeToGoMode(uint32(unixMode))
}

// formatFileMode converts a Go fs.FileMode to Unix mode string
func formatFileMode(mode fs.FileMode) string {
	unixMode := goModeToUnixMode(mode)
	return strconv.FormatUint(uint64(unixMode), 10)
}

// unixModeToGoMode converts Unix file mode to Go fs.FileMode
func unixModeToGoMode(unixMode uint32) fs.FileMode {
	// Extract permission bits (lower 9 bits)
	perm := fs.FileMode(unixMode & 0o777)

	// Extract file type from Unix mode
	switch unixMode & 0o170000 { // S_IFMT mask
	case 0o40000: // S_IFDIR - directory
		return fs.ModeDir | perm
	case 0o120000: // S_IFLNK - symbolic link
		return fs.ModeSymlink | perm
	case 0o60000: // S_IFBLK - block device
		return fs.ModeDevice | perm
	case 0o20000: // S_IFCHR - character device
		return fs.ModeCharDevice | perm
	case 0o10000: // S_IFIFO - named pipe (FIFO)
		return fs.ModeNamedPipe | perm
	case 0o140000: // S_IFSOCK - socket
		return fs.ModeSocket | perm
	case 0o100000: // S_IFREG - regular file
		fallthrough
	default:
		return perm
	}
}

// goModeToUnixMode converts Go fs.FileMode to Unix file mode
func goModeToUnixMode(mode fs.FileMode) uint32 {
	// Start with permission bits
	unixMode := uint32(mode & fs.ModePerm) // 0o777

	// Add file type bits
	if mode&fs.ModeDir != 0 {
		unixMode |= 0o40000 // S_IFDIR
	} else if mode&fs.ModeSymlink != 0 {
		unixMode |= 0o120000 // S_IFLNK
	} else if mode&fs.ModeDevice != 0 {
		unixMode |= 0o60000 // S_IFBLK
	} else if mode&fs.ModeCharDevice != 0 {
		unixMode |= 0o20000 // S_IFCHR
	} else if mode&fs.ModeNamedPipe != 0 {
		unixMode |= 0o10000 // S_IFIFO
	} else if mode&fs.ModeSocket != 0 {
		unixMode |= 0o140000 // S_IFSOCK
	} else {
		unixMode |= 0o100000 // S_IFREG - regular file
	}

	return unixMode
}

// parseModTime converts a timestamp string to time.Time
func parseModTime(timeStr string) time.Time {
	if timeStr == "" {
		return time.Time{}
	}
	timestamp, err := strconv.ParseInt(timeStr, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(timestamp, 0)
}

// parseOwnership extracts uid and gid from ownership string
func parseOwnership(ownerStr string) (uid, gid int) {
	if ownerStr == "" {
		return 0, 0
	}
	parts := strings.Split(ownerStr, ":")
	if len(parts) != 2 {
		return 0, 0
	}
	uid, _ = strconv.Atoi(parts[0])
	gid, _ = strconv.Atoi(parts[1])
	return uid, gid
}

// getCachedHead retrieves cached HEAD request result if valid
func (fsys *FS) getCachedHead(path string) (*r2FileInfo, error, bool) {
	fsys.cacheMu.RLock()
	defer fsys.cacheMu.RUnlock()

	entry, exists := fsys.headCache[path]
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

	// Return a copy to prevent external modification
	infoCopy := *entry.info
	return &infoCopy, nil, true
}

// setCachedHead stores HEAD request result in cache
func (fsys *FS) setCachedHead(path string, info *r2FileInfo) {
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()

	now := time.Now()
	fsys.headCache[path] = &headCacheEntry{
		info:      info,
		err:       nil,
		cachedAt:  now,
		expiresAt: now.Add(fsys.cacheTTL),
	}
}

// setCachedHeadError stores HEAD request error in cache
func (fsys *FS) setCachedHeadError(path string, err error) {
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()

	now := time.Now()
	fsys.headCache[path] = &headCacheEntry{
		info:      nil,
		err:       err,
		cachedAt:  now,
		expiresAt: now.Add(fsys.cacheTTL),
	}
}

// invalidateCachedHead removes cached HEAD request result
func (fsys *FS) invalidateCachedHead(path string) {
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()
	delete(fsys.headCache, path)
}

// SetCacheTTL sets the cache TTL for HEAD requests
func (fsys *FS) SetCacheTTL(ttl time.Duration) {
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()
	fsys.cacheTTL = ttl
}

// GetCacheTTL returns the current cache TTL
func (fsys *FS) GetCacheTTL() time.Duration {
	fsys.cacheMu.RLock()
	defer fsys.cacheMu.RUnlock()
	return fsys.cacheTTL
}

// ClearHeadCache clears all cached HEAD request results
func (fsys *FS) ClearHeadCache() {
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()
	fsys.headCache = make(map[string]*headCacheEntry)
}

// ExpireOldHeadCache removes expired entries from the cache
func (fsys *FS) ExpireOldHeadCache() int {
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()

	now := time.Now()
	expired := 0

	for path, entry := range fsys.headCache {
		if now.After(entry.expiresAt) {
			delete(fsys.headCache, path)
			expired++
		}
	}

	return expired
}

// SetBasePath sets the base path that will be prepended to all R2 object keys
func (fsys *FS) SetBasePath(basePath string) {
	fsys.cacheMu.Lock()
	defer fsys.cacheMu.Unlock()
	fsys.basePath = basePath
	// Clear cache since paths have changed
	fsys.headCache = make(map[string]*headCacheEntry)
}

// GetBasePath returns the current base path
func (fsys *FS) GetBasePath() string {
	fsys.cacheMu.RLock()
	defer fsys.cacheMu.RUnlock()
	return fsys.basePath
}

// headRequest performs a HEAD request to get file metadata
func (fsys *FS) headRequest(ctx context.Context, path string) (*r2FileInfo, error) {
	// Check cache first
	if cachedInfo, cachedErr, found := fsys.getCachedHead(path); found {
		if cachedErr != nil {
			return nil, cachedErr
		}
		return cachedInfo, nil
	}

	objectKey := fsys.normalizeR2Path(path)

	// Use S3 HeadObject to get metadata
	input := &s3.HeadObjectInput{
		Bucket: aws.String(fsys.bucketName),
		Key:    aws.String(objectKey),
	}

	resp, err := fsys.client.HeadObject(ctx, input)
	if err != nil {
		// Check if it's a not found error
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchKey") {
			// Special case: if this is the root directory within a base path,
			// treat it as an implicit directory that always exists
			if fsys.basePath != "" && (path == "." || path == "") {
				info := &r2FileInfo{
					name:    ".",
					size:    0,
					mode:    fs.ModeDir | 0755,
					modTime: time.Now(),
					isDir:   true,
				}
				fsys.setCachedHead(path, info)
				return info, nil
			}

			fsys.setCachedHeadError(path, fs.ErrNotExist)
			return nil, fs.ErrNotExist
		}
		return nil, err
	}

	info := fsys.parseFileInfo(path, resp.Metadata, *resp.ContentLength)

	// Cache the result
	fsys.setCachedHead(path, info)

	return info, nil
}

// getMetadataValue performs case-insensitive lookup in metadata map
func getMetadataValue(metadata map[string]string, key string) string {
	// Try exact match first
	if val, ok := metadata[key]; ok {
		return val
	}
	// Try case-insensitive search
	lowerKey := strings.ToLower(key)
	for k, v := range metadata {
		if strings.ToLower(k) == lowerKey {
			return v
		}
	}
	return ""
}

// parseFileInfo extracts file information from R2 metadata
func (fsys *FS) parseFileInfo(path string, metadata map[string]string, size int64) *r2FileInfo {
	name := filepath.Base(path)

	mode := parseFileMode(getMetadataValue(metadata, "Content-Mode"))
	modTime := parseModTime(getMetadataValue(metadata, "Content-Modified"))
	isDir := getMetadataValue(metadata, "Content-Type") == "application/x-directory"

	// Set default modes if not provided
	if mode == 0 {
		if isDir {
			mode = fs.ModeDir | 0755
		} else {
			mode = 0644
		}
	}

	// Check if mode indicates directory
	if isDir && (mode&fs.ModeDir == 0) {
		// If Content-Type says directory but mode doesn't, add the directory flag
		mode = fs.ModeDir | (mode & fs.ModePerm)
	}

	return &r2FileInfo{
		name:    name,
		size:    size,
		mode:    mode,
		modTime: modTime,
		isDir:   isDir,
	}
}

// Core filesystem interface implementations

// Open opens the named file for reading
func (fsys *FS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

// OpenContext opens the named file for reading with context
func (fsys *FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	objectKey := fsys.normalizeR2Path(name)

	// Use S3 GetObject to get content
	input := &s3.GetObjectInput{
		Bucket: aws.String(fsys.bucketName),
		Key:    aws.String(objectKey),
	}

	resp, err := fsys.client.GetObject(ctx, input)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchKey") {
			// Special case: if this is the root directory within a base path,
			// treat it as an implicit empty directory that always exists
			if fsys.basePath != "" && (name == "." || name == "") {
				fileInfo := &r2FileInfo{
					name:    ".",
					size:    0,
					mode:    fs.ModeDir | 0755,
					modTime: time.Now(),
					isDir:   true,
				}
				return &r2File{
					fs:       fsys,
					path:     name,
					isDir:    true,
					fileInfo: fileInfo,
					content:  []byte{}, // Empty directory content
					pos:      0,
					closed:   false,
				}, nil
			}
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fileInfo := fsys.parseFileInfo(name, resp.Metadata, *resp.ContentLength)

	return &r2File{
		fs:       fsys,
		path:     name,
		isDir:    fileInfo.isDir,
		fileInfo: fileInfo,
		content:  content,
		pos:      0,
		closed:   false,
	}, nil
}

// Create creates or truncates the named file
func (fsys *FS) Create(name string) (fs.File, error) {
	return fsys.createFile(context.Background(), name, nil, 0644)
}

// createFile is a helper for creating files with content and mode
func (fsys *FS) createFile(ctx context.Context, name string, content []byte, mode fs.FileMode) (fs.File, error) {
	objectKey := fsys.normalizeR2Path(name)

	// Use compare-and-swap for atomic creation with timestamp ordering
	err := fsys.compareAndSwap(ctx, objectKey, func(existingContent []byte, existingMetadata map[string]string, etag string) ([]byte, map[string]string, error) {
		// Prepare metadata according to R2 filesystem design
		metadata := map[string]string{
			"Content-Mode":      formatFileMode(mode),
			"Content-Modified":  strconv.FormatInt(time.Now().Unix(), 10),
			"Content-Ownership": "0:0",
			"Change-Timestamp":  strconv.FormatInt(time.Now().UnixMicro(), 10),
		}

		// If object exists, check timestamp ordering
		if existingMetadata != nil {
			existingTimestamp := parseMicroseconds(getMetadataValue(existingMetadata, "Change-Timestamp"))
			newTimestamp := parseMicroseconds(getMetadataValue(metadata, "Change-Timestamp"))
			if newTimestamp <= existingTimestamp {
				// Keep existing metadata if timestamp is not newer
				return existingContent, existingMetadata, nil
			}
		}

		return content, metadata, nil
	})
	if err != nil {
		return nil, err
	}

	// Update parent directory listing
	err = fsys.updateParentDirectoryListing(ctx, name, formatFileMode(mode), false)
	if err != nil {
		// If parent directory update fails, try to clean up the created file
		fsys.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(fsys.bucketName),
			Key:    aws.String(objectKey),
		})
		return nil, err
	}

	// Invalidate cache for this path since we modified the file
	fsys.invalidateCachedHead(name)

	// Return a file handle for the created file
	fileInfo := &r2FileInfo{
		name:    filepath.Base(name),
		size:    int64(len(content)),
		mode:    mode,
		modTime: time.Now(),
		isDir:   false,
	}

	// Copy content for the file handle
	fileContent := make([]byte, len(content))
	copy(fileContent, content)

	return &r2File{
		fs:       fsys,
		path:     name,
		isDir:    false,
		fileInfo: fileInfo,
		content:  fileContent,
		pos:      0,
		closed:   false,
	}, nil
}

// R2 File implementation

// Read reads data from the file
func (f *r2File) Read(p []byte) (int, error) {
	if f.closed {
		return 0, fs.ErrClosed
	}

	if f.pos >= int64(len(f.content)) {
		return 0, io.EOF
	}

	n := copy(p, f.content[f.pos:])
	f.pos += int64(n)
	return n, nil
}

// Write writes data to the file
func (f *r2File) Write(p []byte) (int, error) {
	if f.closed {
		return 0, fs.ErrClosed
	}

	// Extend content if necessary
	newLen := f.pos + int64(len(p))
	if newLen > int64(len(f.content)) {
		newContent := make([]byte, newLen)
		copy(newContent, f.content)
		f.content = newContent
	}

	n := copy(f.content[f.pos:], p)
	f.pos += int64(n)

	// Update file info
	f.isDirty = true
	f.fileInfo.size = int64(len(f.content))
	f.fileInfo.modTime = time.Now()
	f.updated = time.Now()

	return n, nil
}

// Seek sets the offset for the next Read or Write
func (f *r2File) Seek(offset int64, whence int) (int64, error) {
	if f.closed {
		return 0, fs.ErrClosed
	}

	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = f.pos + offset
	case io.SeekEnd:
		newPos = int64(len(f.content)) + offset
	default:
		return 0, fmt.Errorf("invalid whence")
	}

	if newPos < 0 {
		return 0, fmt.Errorf("negative position")
	}

	f.pos = newPos
	return newPos, nil
}

// Close closes the file and saves changes to R2
func (f *r2File) Close() error {
	if f.closed {
		return nil
	}

	f.closed = true

	// Save the file content back to R2 if dirty
	if f.isDirty {
		return f.save()
	}
	return nil
}

// save writes the file content back to R2
func (f *r2File) save() error {
	objectKey := f.fs.normalizeR2Path(f.path)

	// Prepare metadata
	metadata := map[string]string{
		"Content-Mode":      formatFileMode(f.fileInfo.mode),
		"Content-Modified":  strconv.FormatInt(f.fileInfo.modTime.Unix(), 10),
		"Content-Ownership": "0:0",
		"Change-Timestamp":  strconv.FormatInt(f.updated.UnixMicro(), 10),
	}

	var contentType string
	if f.isDir {
		contentType = "application/x-directory"
	} else {
		contentType = "application/octet-stream"
	}

	// Use S3 PutObject to save the file
	input := &s3.PutObjectInput{
		Bucket:      aws.String(f.fs.bucketName),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(f.content),
		Metadata:    metadata,
		ContentType: aws.String(contentType),
	}

	_, err := f.fs.client.PutObject(context.Background(), input)
	if err != nil {
		return err
	}

	// Invalidate cache for this path since we saved the file
	f.fs.invalidateCachedHead(f.path)

	return nil
}

// Stat returns file information
func (f *r2File) Stat() (fs.FileInfo, error) {
	if f.closed {
		return nil, fs.ErrClosed
	}
	return f.fileInfo, nil
}

// ReadDir reads directory entries (for directories)
func (f *r2File) ReadDir(count int) ([]fs.DirEntry, error) {
	if f.closed {
		return nil, fs.ErrClosed
	}

	if !f.isDir {
		return nil, fmt.Errorf("not a directory")
	}

	if f.entries == nil {
		entries, err := f.parseDirectoryListing()
		if err != nil {
			return nil, err
		}
		f.entries = entries
	}

	if count == -1 {
		defer func() {
			f.entries = nil
			f.dirPos = 0
		}()
		return f.entries, nil
	}

	n := len(f.entries) - f.dirPos
	if n == 0 && count > 0 {
		return nil, io.EOF
	}

	if count > 0 && n > count {
		n = count
	}

	list := make([]fs.DirEntry, n)
	for i := range list {
		list[i] = f.entries[f.dirPos+i]
	}
	f.dirPos += n

	return list, nil
}

// parseDirectoryListing parses the directory content format according to R2 spec
func (f *r2File) parseDirectoryListing() ([]fs.DirEntry, error) {
	content := string(f.content)
	lines := strings.Split(strings.TrimSpace(content), "\n")

	var entries []fs.DirEntry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}

		name := parts[0]
		modeStr := parts[1]
		mode := parseFileMode(modeStr)

		isDir := mode&fs.ModeDir != 0

		info := &r2FileInfo{
			name:  name,
			mode:  mode,
			isDir: isDir,
		}

		entries = append(entries, fs.FileInfoToDirEntry(info))
	}

	return entries, nil
}

// Mkdir creates a directory
func (fsys *FS) Mkdir(name string, perm fs.FileMode) error {
	return fsys.mkdir(context.Background(), name, perm)
}

// mkdir creates a directory with context
func (fsys *FS) mkdir(ctx context.Context, name string, perm fs.FileMode) error {
	objectKey := fsys.normalizeR2Path(name)

	// Prepare metadata for directory
	metadata := map[string]string{
		"Content-Mode":      formatFileMode(perm | fs.ModeDir),
		"Content-Modified":  strconv.FormatInt(time.Now().Unix(), 10),
		"Content-Ownership": "0:0",
		"Change-Timestamp":  strconv.FormatInt(time.Now().UnixMicro(), 10),
	}

	// Create empty directory with empty content
	input := &s3.PutObjectInput{
		Bucket:      aws.String(fsys.bucketName),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader([]byte{}),
		Metadata:    metadata,
		ContentType: aws.String("application/x-directory"),
	}

	_, err := fsys.client.PutObject(ctx, input)
	if err != nil {
		return err
	}

	// Update parent directory listing
	err = fsys.updateParentDirectoryListing(ctx, name, formatFileMode(perm|fs.ModeDir), false)
	if err != nil {
		// If parent directory update fails, try to clean up the created directory
		fsys.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(fsys.bucketName),
			Key:    aws.String(objectKey),
		})
		return err
	}

	// Invalidate cache for this path since we created a directory
	fsys.invalidateCachedHead(name)

	return nil
}

// Remove removes a file or empty directory
func (fsys *FS) Remove(name string) error {
	return fsys.remove(context.Background(), name)
}

// remove removes a file or directory with context
func (fsys *FS) remove(ctx context.Context, name string) error {
	objectKey := fsys.normalizeR2Path(name)

	// First, get the object to check if it's a directory
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(fsys.bucketName),
		Key:    aws.String(objectKey),
	}

	resp, err := fsys.client.GetObject(ctx, getInput)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchKey") {
			return fs.ErrNotExist
		}
		return err
	}

	isDirectory := false
	if resp.ContentType != nil && *resp.ContentType == "application/x-directory" {
		isDirectory = true
	}

	// If it's a directory, recursively delete its contents
	if isDirectory {
		content, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		entries := parseDirectoryEntries(string(content))
		for entryName := range entries {
			// Compose sub path (fs interface format)
			var subPath string
			if name == "." {
				subPath = entryName
			} else {
				subPath = name + "/" + entryName
			}

			// Recursively delete each entry
			err = fsys.remove(ctx, subPath)
			if err != nil {
				return err
			}
		}
	} else {
		resp.Body.Close()
	}

	// Remove from parent directory listing
	err = fsys.updateParentDirectoryListing(ctx, name, "", true)
	if err != nil {
		return err
	}

	// Use S3 DeleteObject to remove the file/directory
	deleteInput := &s3.DeleteObjectInput{
		Bucket: aws.String(fsys.bucketName),
		Key:    aws.String(objectKey),
	}

	_, err = fsys.client.DeleteObject(ctx, deleteInput)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchKey") {
			return fs.ErrNotExist
		}
		return err
	}

	// Invalidate cache for this path since we removed the file
	fsys.invalidateCachedHead(name)

	return nil
}

// StatContext returns file information
func (fsys *FS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	return fsys.headRequest(ctx, name)
}

// Stat returns file information
func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	return fsys.StatContext(context.Background(), name)
}

// Chmod changes the mode of the named file
func (fsys *FS) Chmod(name string, mode fs.FileMode) error {
	return fsys.chmod(context.Background(), name, mode)
}

// chmod changes file mode with context
func (fsys *FS) chmod(ctx context.Context, name string, mode fs.FileMode) error {
	objectKey := fsys.normalizeR2Path(name)

	// First, get the current object to preserve other metadata
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(fsys.bucketName),
		Key:    aws.String(objectKey),
	}

	resp, err := fsys.client.GetObject(ctx, getInput)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchKey") {
			return fs.ErrNotExist
		}
		return err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Update metadata with new mode
	metadata := resp.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["Content-Mode"] = formatFileMode(mode)
	metadata["Change-Timestamp"] = strconv.FormatInt(time.Now().UnixMicro(), 10)

	// Put the object back with updated metadata
	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(fsys.bucketName),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(content),
		Metadata:    metadata,
		ContentType: resp.ContentType,
	}

	_, err = fsys.client.PutObject(ctx, putInput)
	if err != nil {
		return err
	}

	// Invalidate cache for this path since we changed the mode
	fsys.invalidateCachedHead(name)

	return nil
}

// Chown changes the numeric uid and gid of the named file
func (fsys *FS) Chown(name string, uid, gid int) error {
	return fsys.chown(context.Background(), name, uid, gid)
}

// chown changes ownership with context
func (fsys *FS) chown(ctx context.Context, name string, uid, gid int) error {
	objectKey := fsys.normalizeR2Path(name)

	// First, get the current object to preserve other metadata
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(fsys.bucketName),
		Key:    aws.String(objectKey),
	}

	resp, err := fsys.client.GetObject(ctx, getInput)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchKey") {
			return fs.ErrNotExist
		}
		return err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Update metadata with new ownership
	metadata := resp.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	ownership := fmt.Sprintf("%d:%d", uid, gid)
	metadata["Content-Ownership"] = ownership
	metadata["Change-Timestamp"] = strconv.FormatInt(time.Now().UnixMicro(), 10)

	// Put the object back with updated metadata
	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(fsys.bucketName),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(content),
		Metadata:    metadata,
		ContentType: resp.ContentType,
	}

	_, err = fsys.client.PutObject(ctx, putInput)
	if err != nil {
		return err
	}

	// Invalidate cache for this path since we changed the ownership
	fsys.invalidateCachedHead(name)

	return nil
}

// Chtimes changes the access and modification times of the named file
func (fsys *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return fsys.chtimes(context.Background(), name, atime, mtime)
}

// chtimes changes times with context
func (fsys *FS) chtimes(ctx context.Context, name string, atime time.Time, mtime time.Time) error {
	objectKey := fsys.normalizeR2Path(name)

	// First, get the current object to preserve other metadata
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(fsys.bucketName),
		Key:    aws.String(objectKey),
	}

	resp, err := fsys.client.GetObject(ctx, getInput)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchKey") {
			return fs.ErrNotExist
		}
		return err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Update metadata with new modification time (R2 filesystem design only supports mtime)
	metadata := resp.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["Content-Modified"] = strconv.FormatInt(mtime.Unix(), 10)
	metadata["Change-Timestamp"] = strconv.FormatInt(time.Now().UnixMicro(), 10)

	// Put the object back with updated metadata
	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(fsys.bucketName),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(content),
		Metadata:    metadata,
		ContentType: resp.ContentType,
	}

	_, err = fsys.client.PutObject(ctx, putInput)
	if err != nil {
		return err
	}

	// Invalidate cache for this path since we changed the modification time
	fsys.invalidateCachedHead(name)

	return nil
}

// Symlink creates a symbolic link
func (fsys *FS) Symlink(oldname, newname string) error {
	return fsys.symlink(context.Background(), oldname, newname)
}

// symlink creates a symbolic link with context
func (fsys *FS) symlink(ctx context.Context, oldname, newname string) error {
	objectKey := fsys.normalizeR2Path(newname)

	// Prepare metadata for symlink
	metadata := map[string]string{
		"Content-Mode":      formatFileMode(fs.ModeSymlink | 0777),
		"Content-Modified":  strconv.FormatInt(time.Now().Unix(), 10),
		"Content-Ownership": "0:0",
		"Change-Timestamp":  strconv.FormatInt(time.Now().UnixMicro(), 10),
	}

	// Create symlink with target path as content
	input := &s3.PutObjectInput{
		Bucket:      aws.String(fsys.bucketName),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader([]byte(oldname)),
		Metadata:    metadata,
		ContentType: aws.String("application/x-symlink"),
	}

	_, err := fsys.client.PutObject(ctx, input)
	if err != nil {
		return err
	}

	// Update parent directory listing
	err = fsys.updateParentDirectoryListing(ctx, newname, formatFileMode(fs.ModeSymlink|0777), false)
	if err != nil {
		// If parent directory update fails, try to clean up the created symlink
		fsys.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(fsys.bucketName),
			Key:    aws.String(objectKey),
		})
		return err
	}

	// Invalidate cache for this path since we created a symlink
	fsys.invalidateCachedHead(newname)

	return nil
}

// Readlink reads the target of a symbolic link
func (fsys *FS) Readlink(name string) (string, error) {
	return fsys.readlink(context.Background(), name)
}

// readlink reads the target of a symbolic link with context
func (fsys *FS) readlink(ctx context.Context, name string) (string, error) {
	objectKey := fsys.normalizeR2Path(name)

	// Use S3 GetObject to get symlink content
	input := &s3.GetObjectInput{
		Bucket: aws.String(fsys.bucketName),
		Key:    aws.String(objectKey),
	}

	resp, err := fsys.client.GetObject(ctx, input)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchKey") {
			return "", fs.ErrNotExist
		}
		return "", err
	}
	defer resp.Body.Close()

	// Check if it's actually a symlink
	if resp.ContentType != nil && *resp.ContentType != "application/x-symlink" {
		return "", fmt.Errorf("expected Content-Type application/x-symlink, got %s", *resp.ContentType)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// Rename moves/renames a file or directory
func (fsys *FS) Rename(oldname, newname string) error {
	return fsys.rename(context.Background(), oldname, newname)
}

// rename moves/renames a file or directory with context
func (fsys *FS) rename(ctx context.Context, oldname, newname string) error {
	oldObjectKey := fsys.normalizeR2Path(oldname)
	newObjectKey := fsys.normalizeR2Path(newname)

	// First, get the current object
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(fsys.bucketName),
		Key:    aws.String(oldObjectKey),
	}

	resp, err := fsys.client.GetObject(ctx, getInput)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchKey") {
			return fs.ErrNotExist
		}
		return err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Update metadata with new change timestamp
	metadata := resp.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["Change-Timestamp"] = strconv.FormatInt(time.Now().UnixMicro(), 10)

	// Get the file mode for directory listing updates
	fileMode := getMetadataValue(metadata, "Content-Mode")
	if fileMode == "" {
		fileMode = "33188" // Default file mode
	}

	// Add to destination parent directory listing
	err = fsys.updateParentDirectoryListing(ctx, newname, fileMode, false)
	if err != nil {
		return err
	}

	// Create the object at the new location
	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(fsys.bucketName),
		Key:         aws.String(newObjectKey),
		Body:        bytes.NewReader(content),
		Metadata:    metadata,
		ContentType: resp.ContentType,
	}

	_, err = fsys.client.PutObject(ctx, putInput)
	if err != nil {
		// If put fails, try to clean up the destination directory listing
		fsys.updateParentDirectoryListing(ctx, newname, "", true)
		return err
	}

	// If the source is a directory, recursively move its contents
	if resp.ContentType != nil && *resp.ContentType == "application/x-directory" {
		entries := parseDirectoryEntries(string(content))
		for entryName := range entries {
			// Compose child source and destination paths (fs interface format)
			var childSrc, childDest string
			if oldname == "." {
				childSrc = entryName
			} else {
				childSrc = oldname + "/" + entryName
			}
			if newname == "." {
				childDest = entryName
			} else {
				childDest = newname + "/" + entryName
			}

			// Recursively move each entry
			err = fsys.rename(ctx, childSrc, childDest)
			if err != nil {
				return err
			}
		}
	}

	// Remove from source parent directory listing
	err = fsys.updateParentDirectoryListing(ctx, oldname, "", true)
	if err != nil {
		// If source directory update fails, try to clean up the new object and destination listing
		fsys.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(fsys.bucketName),
			Key:    aws.String(newObjectKey),
		})
		fsys.updateParentDirectoryListing(ctx, newname, "", true)
		return err
	}

	// Delete the old object
	deleteInput := &s3.DeleteObjectInput{
		Bucket: aws.String(fsys.bucketName),
		Key:    aws.String(oldObjectKey),
	}

	_, err = fsys.client.DeleteObject(ctx, deleteInput)
	if err != nil {
		// If delete fails, this is a problem but we've already updated directories
		// Log the error but don't fail the operation
		return err
	}

	// Invalidate cache for both old and new paths since we moved the file
	fsys.invalidateCachedHead(oldname)
	fsys.invalidateCachedHead(newname)

	return nil
}

// Patch updates only metadata for a file or directory without changing content
func (fsys *FS) Patch(name string, metadata map[string]string) error {
	return fsys.patch(context.Background(), name, metadata)
}

// patch updates metadata with context
func (fsys *FS) patch(ctx context.Context, name string, newMetadata map[string]string) error {
	objectKey := fsys.normalizeR2Path(name)

	// Use compare-and-swap to update metadata atomically
	return fsys.compareAndSwap(ctx, objectKey, func(content []byte, metadata map[string]string, etag string) ([]byte, map[string]string, error) {
		// Merge new metadata with existing metadata
		if metadata == nil {
			metadata = make(map[string]string)
		}

		// Update change timestamp
		metadata["Change-Timestamp"] = strconv.FormatInt(time.Now().UnixMicro(), 10)

		// Merge in new metadata
		for key, value := range newMetadata {
			metadata[key] = value
		}

		// If Content-Mode is being updated, also update parent directory listing
		if newMode, hasMode := newMetadata["Content-Mode"]; hasMode && name != "/" {
			// Update parent directory listing with new mode
			err := fsys.updateParentDirectoryListing(ctx, name, newMode, false)
			if err != nil {
				return nil, nil, err
			}
		}

		// Return same content with updated metadata
		return content, metadata, nil
	})
}

// Copy copies a file or directory to a new location
func (fsys *FS) Copy(oldname, newname string, overwrite bool) error {
	return fsys.copy(context.Background(), oldname, newname, overwrite)
}

// copy copies a file or directory with context
func (fsys *FS) copy(ctx context.Context, oldname, newname string, overwrite bool) error {
	if oldname == newname {
		return fmt.Errorf("cannot copy to same path")
	}

	if newname == "/" {
		return fmt.Errorf("cannot copy to root")
	}

	oldObjectKey := fsys.normalizeR2Path(oldname)
	newObjectKey := fsys.normalizeR2Path(newname)

	// Check if destination exists and overwrite is not allowed
	if !overwrite {
		_, err := fsys.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(fsys.bucketName),
			Key:    aws.String(newObjectKey),
		})
		if err == nil {
			return fmt.Errorf("destination exists and overwrite is false")
		}
	}

	// Get source object
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(fsys.bucketName),
		Key:    aws.String(oldObjectKey),
	}

	resp, err := fsys.client.GetObject(ctx, getInput)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NoSuchKey") {
			return fs.ErrNotExist
		}
		return err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Get the file mode for directory listing updates
	fileMode := getMetadataValue(resp.Metadata, "Content-Mode")
	if fileMode == "" {
		fileMode = "33188" // Default file mode
	}

	// Add to destination parent directory listing
	err = fsys.updateParentDirectoryListing(ctx, newname, fileMode, false)
	if err != nil {
		return err
	}

	// Create the object at the new location
	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(fsys.bucketName),
		Key:         aws.String(newObjectKey),
		Body:        bytes.NewReader(content),
		Metadata:    resp.Metadata,
		ContentType: resp.ContentType,
	}

	_, err = fsys.client.PutObject(ctx, putInput)
	if err != nil {
		// If put fails, try to clean up the destination directory listing
		fsys.updateParentDirectoryListing(ctx, newname, "", true)
		return err
	}

	// If the source is a directory, recursively copy its contents
	if resp.ContentType != nil && *resp.ContentType == "application/x-directory" {
		entries := parseDirectoryEntries(string(content))
		for entryName := range entries {
			// Compose child source and destination paths (fs interface format)
			var childSrc, childDest string
			if oldname == "." {
				childSrc = entryName
			} else {
				childSrc = oldname + "/" + entryName
			}
			if newname == "." {
				childDest = entryName
			} else {
				childDest = newname + "/" + entryName
			}

			// Recursively copy each entry
			err = fsys.copy(ctx, childSrc, childDest, overwrite)
			if err != nil {
				return err
			}
		}
	}

	// Invalidate cache for the new path
	fsys.invalidateCachedHead(newname)

	return nil
}

// Helper functions for directory listing management

// basename returns the final element of a path (handles fs interface paths)
func basename(path string) string {
	// Handle root case
	if path == "." || path == "" {
		return "."
	}

	// Convert to R2 path format for processing
	r2Path := path
	if path != "." && !strings.HasPrefix(path, "/") {
		r2Path = "/" + path
	}

	if r2Path == "/" {
		return "."
	}

	r2Path = strings.TrimSuffix(r2Path, "/")
	lastSlash := strings.LastIndex(r2Path, "/")
	if lastSlash == -1 {
		return r2Path
	}
	return r2Path[lastSlash+1:]
}

// dirpath returns the directory portion of a path (handles fs interface paths)
func dirpath(path string) string {
	// Handle root case
	if path == "." || path == "" {
		return "."
	}

	// Convert to R2 path format for processing
	r2Path := path
	if path != "." && !strings.HasPrefix(path, "/") {
		r2Path = "/" + path
	}

	r2Path = strings.TrimSuffix(r2Path, "/")
	lastSlash := strings.LastIndex(r2Path, "/")
	if lastSlash <= 0 {
		return "." // Return "." for root in fs interface
	}

	dirR2Path := r2Path[:lastSlash]
	if dirR2Path == "/" {
		return "." // Root directory is "." in fs interface
	}

	// Convert back to fs interface format (remove leading /)
	return dirR2Path[1:]
}

// parseDirectoryEntries parses directory listing content into a map
func parseDirectoryEntries(content string) map[string]string {
	entries := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[0]
			mode := parts[1]
			entries[name] = mode
		}
	}

	return entries
}

// formatDirectoryEntries formats directory entries map into listing content
func formatDirectoryEntries(entries map[string]string) string {
	if len(entries) == 0 {
		return ""
	}

	// Sort entries by name
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)

	var lines []string
	for _, name := range names {
		mode := entries[name]
		lines = append(lines, fmt.Sprintf("%s %s", name, mode))
	}

	return strings.Join(lines, "\n") + "\n"
}

// compareAndSwap performs atomic compare-and-swap operations using ETags
func (fsys *FS) compareAndSwap(ctx context.Context, key string, updateFn func(content []byte, metadata map[string]string, etag string) ([]byte, map[string]string, error)) error {
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		// Get current object with ETag
		input := &s3.GetObjectInput{
			Bucket: aws.String(fsys.bucketName),
			Key:    aws.String(key),
		}

		resp, err := fsys.client.GetObject(ctx, input)
		if err != nil {
			return err
		}

		content, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		etag := ""
		if resp.ETag != nil {
			etag = *resp.ETag
		}

		// Apply update function
		newContent, newMetadata, err := updateFn(content, resp.Metadata, etag)
		if err != nil {
			return err
		}

		// Try to write with ETag condition
		putInput := &s3.PutObjectInput{
			Bucket:   aws.String(fsys.bucketName),
			Key:      aws.String(key),
			Body:     bytes.NewReader(newContent),
			Metadata: newMetadata,
		}

		if etag != "" {
			putInput.IfMatch = aws.String(etag)
		}

		if resp.ContentType != nil {
			putInput.ContentType = resp.ContentType
		}

		_, err = fsys.client.PutObject(ctx, putInput)
		if err != nil {
			// Check if it's a precondition failed error (ETag mismatch)
			if strings.Contains(err.Error(), "PreconditionFailed") && i < maxRetries-1 {
				// Wait and retry
				time.Sleep(time.Duration(i*100) * time.Millisecond)
				continue
			}
			return err
		}

		// Success
		return nil
	}

	return fmt.Errorf("compare-and-swap failed after %d retries", maxRetries)
}

// updateParentDirectoryListing updates the parent directory's listing when files are added/removed
func (fsys *FS) updateParentDirectoryListing(ctx context.Context, path, mode string, isDelete bool) error {
	if path == "." || path == "" {
		return nil // Root has no parent
	}

	parentDir := dirpath(path)
	fileName := basename(path)
	parentKey := fsys.normalizeR2Path(parentDir)

	return fsys.compareAndSwap(ctx, parentKey, func(content []byte, metadata map[string]string, etag string) ([]byte, map[string]string, error) {
		entries := parseDirectoryEntries(string(content))

		if isDelete {
			delete(entries, fileName)
		} else {
			entries[fileName] = mode
		}

		newContent := formatDirectoryEntries(entries)

		// Update change timestamp
		if metadata == nil {
			metadata = make(map[string]string)
		}
		metadata["Change-Timestamp"] = strconv.FormatInt(time.Now().UnixMicro(), 10)

		return []byte(newContent), metadata, nil
	})
}

// parseMicroseconds parses an integer string representing microseconds
func parseMicroseconds(str string) int64 {
	if str == "" {
		return 0
	}
	micros, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0
	}
	return micros
}
