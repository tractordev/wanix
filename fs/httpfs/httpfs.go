package httpfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tractor.dev/wanix/fs"
)

// TODO:
// - rename
// - symlink
// - xattrs
// - revisit ownership

// FS implements an HTTP-backed filesystem following the design specification
type FS struct {
	baseURL string
	client  *http.Client
}

// New creates a new HTTP filesystem with the given base URL
func New(baseURL string) *FS {
	return &FS{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{},
	}
}

// NewWithClient creates a new HTTP filesystem with a custom HTTP client
func NewWithClient(baseURL string, client *http.Client) *FS {
	return &FS{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  client,
	}
}

// httpFile represents an HTTP-backed file
type httpFile struct {
	fs       *FS
	path     string
	isDir    bool
	isDirty  bool
	updated  time.Time
	fileInfo *httpFileInfo
	content  []byte
	pos      int64
	closed   bool
	entries  []fs.DirEntry
	dirPos   int
}

// httpFileInfo holds file metadata
type httpFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *httpFileInfo) Name() string       { return fi.name }
func (fi *httpFileInfo) Size() int64        { return fi.size }
func (fi *httpFileInfo) Mode() fs.FileMode  { return fi.mode }
func (fi *httpFileInfo) ModTime() time.Time { return fi.modTime }
func (fi *httpFileInfo) IsDir() bool        { return fi.isDir }
func (fi *httpFileInfo) Sys() interface{}   { return nil }

// normalizeHTTPPath ensures proper HTTP path formatting
func (fsys *FS) normalizeHTTPPath(name string) string {
	var isDir bool
	if strings.HasSuffix(name, "/") {
		isDir = true
	}
	// Clean the path and ensure it starts with /
	name = filepath.Clean("/" + name)
	// Convert backslashes to forward slashes for HTTP
	name = strings.ReplaceAll(name, "\\", "/")
	if isDir {
		name += "/"
	}
	return name
}

// buildURL constructs the full HTTP URL for a path
func (fsys *FS) buildURL(path string) string {
	return fsys.baseURL + fsys.normalizeHTTPPath(path)
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

// headRequest performs a HEAD request to get file metadata
func (fsys *FS) headRequest(ctx context.Context, path string) (*httpFileInfo, error) {
	url := fsys.buildURL(path)

	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := fsys.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fs.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return fsys.parseFileInfo(path, resp.Header), nil
}

// parseFileInfo extracts file information from HTTP headers
func (fsys *FS) parseFileInfo(path string, headers http.Header) *httpFileInfo {
	name := filepath.Base(path)
	size := int64(0)
	if contentLength := headers.Get("Content-Length"); contentLength != "" {
		size, _ = strconv.ParseInt(contentLength, 10, 64)
	}

	mode := parseFileMode(headers.Get("Content-Mode"))
	modTime := parseModTime(headers.Get("Content-Modified"))
	isDir := headers.Get("Content-Type") == "application/x-directory"

	// Set default modes if not provided
	if mode == 0 {
		if isDir {
			mode = fs.ModeDir | 0744
		} else {
			mode = 0644
		}
	}

	// Check if mode indicates directory (parseFileMode should have handled this)
	if isDir && (mode&fs.ModeDir == 0) {
		// If Content-Type says directory but mode doesn't, add the directory flag
		mode = fs.ModeDir | (mode & fs.ModePerm)
	}

	return &httpFileInfo{
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
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := fsys.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fs.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fileInfo := fsys.parseFileInfo(name, resp.Header)

	return &httpFile{
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
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(content))
	if err != nil {
		return nil, err
	}

	// Set headers according to the HTTP filesystem design
	if mode&fs.ModeDir != 0 {
		req.Header.Set("Content-Type", "application/x-directory")
	} else {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	req.Header.Set("Content-Mode", formatFileMode(mode))
	req.Header.Set("Content-Modified", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("Content-Ownership", "0:0")
	req.Header.Set("Content-Length", strconv.Itoa(len(content)))
	req.Header.Set("Change-Timestamp", strconv.FormatInt(time.Now().UnixMicro(), 10))

	resp, err := fsys.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Return a file handle for the created file
	fileInfo := &httpFileInfo{
		name:    filepath.Base(name),
		size:    int64(len(content)),
		mode:    mode,
		modTime: time.Now(),
		isDir:   false,
	}

	// Copy content for the file handle
	fileContent := make([]byte, len(content))
	copy(fileContent, content)

	return &httpFile{
		fs:       fsys,
		path:     name,
		isDir:    false,
		fileInfo: fileInfo,
		content:  fileContent,
		pos:      0,
		closed:   false,
	}, nil
}

func (fsys *FS) Symlink(oldname, newname string) error {
	return fsys.symlink(context.Background(), oldname, newname)
}

func (fsys *FS) symlink(ctx context.Context, oldname, newname string) error {
	url := fsys.buildURL(newname)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader([]byte(oldname)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-symlink")
	req.Header.Set("Content-Mode", formatFileMode(fs.ModeSymlink|0777))
	req.Header.Set("Content-Modified", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("Content-Ownership", "0:0")
	req.Header.Set("Change-Timestamp", strconv.FormatInt(time.Now().UnixMicro(), 10))

	resp, err := fsys.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

func (fsys *FS) Readlink(name string) (string, error) {
	return fsys.readlink(context.Background(), name)
}

func (fsys *FS) readlink(ctx context.Context, name string) (string, error) {
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := fsys.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	if resp.Header.Get("Content-Type") != "application/x-symlink" {
		return "", fmt.Errorf("expected Content-Type application/x-symlink, got %s", resp.Header.Get("Content-Type"))
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (fsys *FS) Rename(oldname, newname string) error {
	return fsys.rename(context.Background(), oldname, newname)
}

func (fsys *FS) rename(ctx context.Context, oldname, newname string) error {
	url := fsys.buildURL(oldname)

	req, err := http.NewRequestWithContext(ctx, "MOVE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Destination", "/"+newname)

	resp, err := fsys.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Mkdir creates a directory
func (fsys *FS) Mkdir(name string, perm fs.FileMode) error {
	return fsys.mkdir(context.Background(), name, perm)
}

// mkdir creates a directory with context
func (fsys *FS) mkdir(ctx context.Context, name string, perm fs.FileMode) error {
	// Ensure the path ends with a slash for directories
	dirPath := name
	if !strings.HasSuffix(dirPath, "/") {
		dirPath += "/"
	}

	url := fsys.buildURL(dirPath)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader([]byte{}))
	if err != nil {
		return err
	}

	// Set directory headers
	req.Header.Set("Content-Type", "application/x-directory")
	req.Header.Set("Content-Mode", formatFileMode(perm|fs.ModeDir))
	req.Header.Set("Content-Modified", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("Content-Ownership", "0:0")
	req.Header.Set("Content-Length", "0")
	req.Header.Set("Change-Timestamp", strconv.FormatInt(time.Now().UnixMicro(), 10))

	resp, err := fsys.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Remove removes a file or empty directory
func (fsys *FS) Remove(name string) error {
	return fsys.remove(context.Background(), name)
}

// remove removes a file or directory with context
func (fsys *FS) remove(ctx context.Context, name string) error {
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := fsys.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fs.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

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
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Mode", formatFileMode(mode))
	req.Header.Set("Change-Timestamp", strconv.FormatInt(time.Now().UnixMicro(), 10))

	resp, err := fsys.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fs.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Chown changes the numeric uid and gid of the named file
func (fsys *FS) Chown(name string, uid, gid int) error {
	return fsys.chown(context.Background(), name, uid, gid)
}

// chown changes ownership with context
func (fsys *FS) chown(ctx context.Context, name string, uid, gid int) error {
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, nil)
	if err != nil {
		return err
	}

	ownership := fmt.Sprintf("%d:%d", uid, gid)
	req.Header.Set("Content-Ownership", ownership)
	req.Header.Set("Change-Timestamp", strconv.FormatInt(time.Now().UnixMicro(), 10))

	resp, err := fsys.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fs.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Chtimes changes the access and modification times of the named file
func (fsys *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return fsys.chtimes(context.Background(), name, atime, mtime)
}

// chtimes changes times with context
func (fsys *FS) chtimes(ctx context.Context, name string, atime time.Time, mtime time.Time) error {
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, nil)
	if err != nil {
		return err
	}

	// HTTP filesystem design only supports modification time
	req.Header.Set("Content-Modified", strconv.FormatInt(mtime.Unix(), 10))
	req.Header.Set("Change-Timestamp", strconv.FormatInt(time.Now().UnixMicro(), 10))

	resp, err := fsys.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fs.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// HTTP File implementation

// Read reads data from the file
func (f *httpFile) Read(p []byte) (int, error) {
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
func (f *httpFile) Write(p []byte) (int, error) {
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
func (f *httpFile) Seek(offset int64, whence int) (int64, error) {
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

// Close closes the file and saves changes to the server
func (f *httpFile) Close() error {
	if f.closed {
		return nil
	}

	f.closed = true

	// Save the file content
	// back to the server if dirty
	if f.isDirty {
		return f.save()
	}
	return nil
}

// save writes the file content back to the HTTP server
func (f *httpFile) save() error {
	url := f.fs.buildURL(f.path)

	req, err := http.NewRequest("PUT", url, bytes.NewReader(f.content))
	if err != nil {
		return err
	}

	// Set headers
	if f.isDir {
		req.Header.Set("Content-Type", "application/x-directory")
	} else {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	req.Header.Set("Content-Length", strconv.Itoa(len(f.content)))
	req.Header.Set("Content-Modified", strconv.FormatInt(f.fileInfo.modTime.Unix(), 10))
	req.Header.Set("Change-Timestamp", strconv.FormatInt(f.updated.UnixMicro(), 10))

	resp, err := f.fs.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Stat returns file information
func (f *httpFile) Stat() (fs.FileInfo, error) {
	if f.closed {
		return nil, fs.ErrClosed
	}
	return f.fileInfo, nil
}

// ReadDir reads directory entries (for directories)
func (f *httpFile) ReadDir(count int) ([]fs.DirEntry, error) {
	if f.closed {
		return nil, fs.ErrClosed
	}

	if !f.isDir {
		log.Println("httpfs: readdir not a directory", f.path)
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
		// f.dirPos = 0
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

// parseDirectoryListing parses the directory content format
func (f *httpFile) parseDirectoryListing() ([]fs.DirEntry, error) {
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

		info := &httpFileInfo{
			name:  name,
			mode:  mode,
			isDir: isDir,
		}

		entries = append(entries, fs.FileInfoToDirEntry(info))
	}

	return entries, nil
}
