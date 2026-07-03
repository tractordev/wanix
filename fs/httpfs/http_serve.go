package httpfs

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/pstat"
)

const (
	protocolMethods = "GET, HEAD, PUT, PATCH, DELETE, MOVE, COPY, OPTIONS"
)

func writeOK(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func writeNotFound(w http.ResponseWriter) {
	http.Error(w, "Object Not Found\n", http.StatusNotFound)
}

func acceptsMultipart(accept string) bool {
	return strings.Contains(accept, "multipart/mixed")
}

// Server implements an HTTP server that serves an fs.FS using the httpfs protocol
type Server struct {
	fs     fs.FS
	prefix string
}

// NewServer creates a new HTTP server for the given filesystem
func NewServer(fsys fs.FS) *Server {
	return &Server{fs: fsys}
}

// NewServerWithPrefix creates a new HTTP server with a URL prefix
func NewServerWithPrefix(fsys fs.FS, prefix string) *Server {
	return &Server{
		fs:     fsys,
		prefix: strings.TrimSuffix(prefix, "/"),
	}
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Strip prefix if present
	path := strings.TrimPrefix(r.URL.Path, s.prefix)
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		path = "."
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGet(w, r, path)
	case http.MethodHead:
		s.handleHead(w, r, path)
	case http.MethodPut:
		s.handlePut(w, r, path)
	case http.MethodDelete:
		s.handleDelete(w, r, path)
	case http.MethodPatch:
		s.handlePatch(w, r, path)
	case "MOVE":
		s.handleMoveCopy(w, r, path, true)
	case "COPY":
		s.handleMoveCopy(w, r, path, false)
	case http.MethodOptions:
		w.Header().Set("Allow", protocolMethods)
		writeOK(w)
	default:
		w.Header().Set("Allow", protocolMethods)
		http.Error(w, "Method Not Allowed\n", http.StatusMethodNotAllowed)
	}
}

// handleGet handles GET requests for files and directories
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, path string) {
	// Check for multipart directory streaming suffixes
	if strings.HasSuffix(path, "/...") {
		// Recursive tree streaming
		path = strings.TrimSuffix(path, "/...")
		s.handleRecursiveStream(w, r, path)
		return
	} else if strings.HasSuffix(path, "/:") {
		// Directory metadata streaming (SPEC extension)
		path = strings.TrimSuffix(path, "/:")
		s.handleDirStream(w, r, path, 1)
		return
	}

	// Check if Accept header requests multipart
	if acceptsMultipart(r.Header.Get("Accept")) {
		info, err := fs.Stat(s.fs, path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				writeNotFound(w)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		if info.IsDir() {
			s.handleDirStream(w, r, path, 2)
			return
		}
	}

	// Normal GET request - use WithNoFollow to get symlink metadata, not target
	ctx := fs.WithNoFollow(r.Context())
	info, err := fs.StatContext(ctx, s.fs, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeNotFound(w)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	s.writeMetadataHeaders(w, info, path)

	// Handle symlinks
	if info.Mode()&fs.ModeSymlink != 0 {
		// For symlinks, return the target path as content
		readlinkFS, ok := s.fs.(interface {
			Readlink(name string) (string, error)
		})
		if ok {
			target, err := readlinkFS.Readlink(path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(target))
			return
		}
	}

	if info.IsDir() {
		listing, err := s.formatDirListing(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Del("Content-Length")
		w.Header().Del("ETag")
		w.WriteHeader(http.StatusOK)
		w.Write(listing)
	} else {
		file, err := s.fs.Open(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		w.WriteHeader(http.StatusOK)
		io.Copy(w, file)
	}
}

// handleHead handles HEAD requests
func (s *Server) handleHead(w http.ResponseWriter, r *http.Request, path string) {
	// Use WithNoFollow to get symlink metadata, not target
	ctx := fs.WithNoFollow(r.Context())
	info, err := fs.StatContext(ctx, s.fs, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeNotFound(w)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	s.writeMetadataHeaders(w, info, path)
	w.WriteHeader(http.StatusOK)
}

// handlePut handles PUT requests for creating/updating files and directories
func (s *Server) handlePut(w http.ResponseWriter, r *http.Request, path string) {
	// Check if this is a directory creation
	contentType := r.Header.Get("Content-Type")
	isDir := contentType == "application/x-directory" || strings.HasSuffix(path, "/")
	isSymlink := contentType == "application/x-symlink"

	if isSymlink {
		// Create symlink
		target, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		symlinkFS, ok := s.fs.(interface {
			Symlink(oldname, newname string) error
		})
		if !ok {
			http.Error(w, "Symlinks not supported", http.StatusNotImplemented)
			return
		}

		if err := symlinkFS.Symlink(string(target), path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeOK(w)
		return
	}

	if isDir {
		path = strings.TrimSuffix(path, "/")
		if path == "" {
			path = "."
		}

		mkdirFS, ok := s.fs.(interface {
			Mkdir(name string, perm fs.FileMode) error
		})
		if !ok {
			http.Error(w, "Directories not supported", http.StatusNotImplemented)
			return
		}

		mode := s.parseModeHeader(r.Header.Get("Content-Mode"), 0755|fs.ModeDir)
		if err := mkdirFS.Mkdir(path, mode); err != nil {
			if info, statErr := fs.Stat(s.fs, path); statErr == nil && info.IsDir() {
				s.updateMetadata(w, r, path)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if r.Header.Get("Content-Mode") != "" || r.Header.Get("Content-Ownership") != "" || r.Header.Get("Content-Modified") != "" {
			s.updateMetadata(w, r, path)
			return
		}

		writeOK(w)
		return
	}

	// Read content first
	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.writeFileContent(path, content, s.parseModeHeader(r.Header.Get("Content-Mode"), 0644)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("Content-Mode") != "" || r.Header.Get("Content-Ownership") != "" || r.Header.Get("Content-Modified") != "" {
		s.updateMetadata(w, r, path)
		return
	}

	writeOK(w)
}

func (s *Server) writeFileContent(path string, content []byte, mode fs.FileMode) error {
	if ns, ok := s.fs.(interface {
		SetNode(name string, node *fskit.Node)
	}); ok {
		ns.SetNode(path, createFileNode(path, content, mode))
		return nil
	}

	createFS, ok := s.fs.(interface {
		Create(name string) (fs.File, error)
	})
	if !ok {
		return fmt.Errorf("file creation not supported")
	}

	file, err := createFS.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if writeFile, ok := file.(interface {
		Write([]byte) (int, error)
	}); ok {
		if _, err := writeFile.Write(content); err != nil {
			return err
		}
	}
	return nil
}

// handleDelete handles DELETE requests
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, path string) {
	if _, err := fs.Stat(s.fs, path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeNotFound(w)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if err := fs.RemoveAll(s.fs, path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeOK(w)
}

// handlePatch handles PATCH requests for metadata updates
func (s *Server) handlePatch(w http.ResponseWriter, r *http.Request, path string) {
	if strings.Contains(r.Header.Get("Content-Type"), "application/x-tar") {
		s.handleTarPatch(w, r, path)
		return
	}
	s.updateMetadata(w, r, path)
}

func (s *Server) handleMoveCopy(w http.ResponseWriter, r *http.Request, path string, move bool) {
	destFS, destHTTP, err := s.parseDestination(r.Header.Get("Destination"))
	if err != nil {
		http.Error(w, err.Error()+"\n", http.StatusBadRequest)
		return
	}

	srcHTTP := s.httpPath(path)
	if strings.TrimSuffix(destHTTP, "/") == strings.TrimSuffix(srcHTTP, "/") {
		http.Error(w, "Cannot move/copy to same path\n", http.StatusBadRequest)
		return
	}

	if _, err := fs.Stat(s.fs, path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "Source Not Found\n", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	overwrite := !strings.EqualFold(r.Header.Get("Overwrite"), "f")
	if _, err := fs.Stat(s.fs, destFS); err == nil && !overwrite {
		http.Error(w, "Destination Exists\n", http.StatusPreconditionFailed)
		return
	}

	if move {
		renameFS, ok := s.fs.(interface {
			Rename(oldname, newname string) error
		})
		if !ok {
			http.Error(w, "Rename not supported", http.StatusNotImplemented)
			return
		}
		if err := renameFS.Rename(path, destFS); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		if err := fs.CopyAll(s.fs, path, destFS); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	writeOK(w)
}

// handleDirStream handles directory metadata streaming with multipart response.
// maxPartDepth controls how deep child parts go (1 = direct children only, 2 = r2fs Accept behavior).
func (s *Server) handleDirStream(w http.ResponseWriter, r *http.Request, path string, maxPartDepth int) {
	info, err := fs.Stat(s.fs, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeNotFound(w)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if !info.IsDir() {
		http.Error(w, "Not a directory", http.StatusBadRequest)
		return
	}

	childPaths, err := s.pathsWithinDepth(path, maxPartDepth)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	dirListing, err := s.formatDirListing(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	part, err := mw.CreatePart(s.createPartHeaders(info, path, int64(len(dirListing))))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	part.Write(dirListing)

	for _, childPath := range childPaths {
		childInfo, err := fs.Stat(s.fs, childPath)
		if err != nil {
			continue
		}

		var partBody []byte
		if childInfo.IsDir() {
			partBody, err = s.formatDirListing(childPath)
			if err != nil {
				continue
			}
		}

		part, err := mw.CreatePart(s.createPartHeaders(childInfo, childPath, int64(len(partBody))))
		if err != nil {
			continue
		}
		if len(partBody) > 0 {
			part.Write(partBody)
		}
	}

	mw.Close()

	w.Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

// handleRecursiveStream handles recursive directory tree streaming
func (s *Server) handleRecursiveStream(w http.ResponseWriter, r *http.Request, path string) {
	info, err := fs.Stat(s.fs, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeNotFound(w)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if !info.IsDir() {
		http.Error(w, "Not a directory", http.StatusBadRequest)
		return
	}

	childPaths, err := s.pathsWithinDepth(path, -1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	dirListing, err := s.formatDirListing(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	part, err := mw.CreatePart(s.createPartHeaders(info, path, int64(len(dirListing))))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	part.Write(dirListing)

	for _, childPath := range childPaths {
		childInfo, err := fs.Stat(s.fs, childPath)
		if err != nil {
			continue
		}

		var partBody []byte
		if childInfo.IsDir() {
			partBody, err = s.formatDirListing(childPath)
			if err != nil {
				continue
			}
		}

		part, err := mw.CreatePart(s.createPartHeaders(childInfo, childPath, int64(len(partBody))))
		if err != nil {
			continue
		}
		if len(partBody) > 0 {
			part.Write(partBody)
		}
	}

	mw.Close()

	w.Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

// writeMetadataHeaders writes filesystem metadata as HTTP headers
func (s *Server) writeMetadataHeaders(w http.ResponseWriter, info fs.FileInfo, path string) {
	w.Header().Set("Content-Location", s.httpPath(path))

	// Content-Type
	if info.IsDir() {
		w.Header().Set("Content-Type", "application/x-directory")
	} else if info.Mode()&fs.ModeSymlink != 0 {
		w.Header().Set("Content-Type", "application/x-symlink")
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	// Content-Length
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))

	// Content-Mode
	unixMode := pstat.FileModeToUnixMode(info.Mode())
	w.Header().Set("Content-Mode", strconv.FormatUint(uint64(unixMode), 10))

	// Content-Modified
	w.Header().Set("Content-Modified", strconv.FormatInt(info.ModTime().Unix(), 10))

	// Content-Ownership (default to 0:0)
	uid, gid := 0, 0
	if sys := info.Sys(); sys != nil {
		if stat, ok := sys.(interface {
			Uid() int
			Gid() int
		}); ok {
			uid = stat.Uid()
			gid = stat.Gid()
		}
	}
	w.Header().Set("Content-Ownership", fmt.Sprintf("%d:%d", uid, gid))
}

// createPartHeaders creates HTTP headers for a multipart part.
// bodyLen is the part body size; 0 means metadata-only with Content-Range.
func (s *Server) createPartHeaders(info fs.FileInfo, path string, bodyLen int64) map[string][]string {
	headers := make(map[string][]string)

	headers["Content-Location"] = []string{s.httpPath(path)}

	// Content-Type
	if info.IsDir() {
		headers["Content-Type"] = []string{"application/x-directory"}
	} else if info.Mode()&fs.ModeSymlink != 0 {
		headers["Content-Type"] = []string{"application/x-symlink"}
	} else {
		headers["Content-Type"] = []string{"application/octet-stream"}
	}

	// Content-Mode
	unixMode := pstat.FileModeToUnixMode(info.Mode())
	headers["Content-Mode"] = []string{strconv.FormatUint(uint64(unixMode), 10)}

	// Content-Modified
	headers["Content-Modified"] = []string{strconv.FormatInt(info.ModTime().Unix(), 10)}

	// Content-Ownership
	uid, gid := 0, 0
	if sys := info.Sys(); sys != nil {
		if stat, ok := sys.(interface {
			Uid() int
			Gid() int
		}); ok {
			uid = stat.Uid()
			gid = stat.Gid()
		}
	}
	headers["Content-Ownership"] = []string{fmt.Sprintf("%d:%d", uid, gid)}

	if bodyLen > 0 {
		headers["Content-Length"] = []string{strconv.FormatInt(bodyLen, 10)}
	} else {
		headers["Content-Range"] = []string{fmt.Sprintf("bytes 0-0/%d", info.Size())}
	}

	return headers
}

// updateMetadata updates file/directory metadata
func (s *Server) updateMetadata(w http.ResponseWriter, r *http.Request, path string) {
	// Check if file exists
	if _, err := fs.Stat(s.fs, path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeNotFound(w)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Update mode
	if modeStr := r.Header.Get("Content-Mode"); modeStr != "" {
		if chmodFS, ok := s.fs.(interface {
			Chmod(name string, mode fs.FileMode) error
		}); ok {
			mode := s.parseModeHeader(modeStr, 0644)
			if err := chmodFS.Chmod(path, mode); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	// Update ownership
	if ownerStr := r.Header.Get("Content-Ownership"); ownerStr != "" {
		if chownFS, ok := s.fs.(interface {
			Chown(name string, uid, gid int) error
		}); ok {
			uid, gid := parseOwnership(ownerStr)
			if err := chownFS.Chown(path, uid, gid); err != nil && !ignorableChownErr(err) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	// Update modification time
	if modTimeStr := r.Header.Get("Content-Modified"); modTimeStr != "" {
		if chtimesFS, ok := s.fs.(interface {
			Chtimes(name string, atime, mtime time.Time) error
		}); ok {
			mtime := parseModTime(modTimeStr)
			if err := chtimesFS.Chtimes(path, mtime, mtime); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	writeOK(w)
}

func (s *Server) httpPath(name string) string {
	if name == "." {
		if s.prefix == "" {
			return "/"
		}
		return s.prefix + "/"
	}
	if s.prefix == "" {
		return "/" + name
	}
	return s.prefix + "/" + name
}

func (s *Server) fsPath(httpPath string) string {
	path := strings.TrimPrefix(httpPath, s.prefix)
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "."
	}
	return path
}

func (s *Server) parseDestination(dest string) (fsPath string, httpPath string, err error) {
	if dest == "" {
		return "", "", fmt.Errorf("Missing or invalid Destination header")
	}
	dest = strings.TrimPrefix(dest, s.prefix)
	if !strings.HasPrefix(dest, "/") {
		return "", "", fmt.Errorf("Missing or invalid Destination header")
	}
	if dest == "/" {
		return "", "", fmt.Errorf("Cannot move/copy to root")
	}
	httpPath = strings.TrimSuffix(dest, "/")
	if httpPath == "" {
		httpPath = "/"
	}
	if s.prefix != "" {
		httpPath = s.prefix + httpPath
	}
	return s.fsPath(dest), httpPath, nil
}

func (s *Server) formatDirListing(path string) ([]byte, error) {
	entries, err := fs.ReadDir(s.fs, path)
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var buf bytes.Buffer
	for _, entry := range entries {
		unixMode := pstat.FileModeToUnixMode(entryMode(entry))
		fmt.Fprintf(&buf, "%s %d\n", entry.Name(), unixMode)
	}
	return buf.Bytes(), nil
}

// pathsWithinDepth returns descendant paths relative to dir, using r2fs depth rules.
func (s *Server) pathsWithinDepth(dir string, maxDepth int) ([]string, error) {
	if maxDepth == 0 {
		return nil, nil
	}

	isRoot := dir == "."
	var paths []string
	var walk func(string) error
	walk = func(current string) error {
		entries, err := fs.ReadDir(s.fs, current)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			child := filepath.Join(current, entry.Name())
			depth := pathPartDepth(dir, child)
			limit := maxDepth
			if isRoot {
				limit = 1
			}
			if maxDepth > 0 && depth > limit {
				continue
			}
			paths = append(paths, child)
			if entry.IsDir() && (maxDepth < 0 || depth < limit) {
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(dir); err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func pathPartDepth(base, child string) int {
	base = filepath.Clean(base)
	child = filepath.Clean(child)
	if base == "." {
		return len(strings.Split(child, "/"))
	}
	rel, err := filepath.Rel(base, child)
	if err != nil || rel == "." {
		return 0
	}
	return len(strings.Split(rel, "/"))
}

// entryMode returns the full file mode for a directory listing entry.
func entryMode(entry fs.DirEntry) fs.FileMode {
	if info, err := entry.Info(); err == nil {
		return info.Mode()
	}
	if entry.IsDir() {
		return fs.ModeDir | 0755
	}
	return 0644
}

// parseModeHeader parses a Content-Mode header value
func (s *Server) parseModeHeader(modeStr string, defaultMode fs.FileMode) fs.FileMode {
	if modeStr == "" {
		return defaultMode
	}
	unixMode, err := strconv.ParseUint(modeStr, 10, 32)
	if err != nil {
		return defaultMode
	}
	return pstat.UnixModeToFileMode(uint32(unixMode))
}

// createFileNode creates a new fskit.Node with the given content
func createFileNode(name string, content []byte, mode fs.FileMode) *fskit.Node {
	// Use Entry which is what memfs.Create uses
	node := fskit.Entry(name, mode, content, time.Now())
	return node
}

func (s *Server) handleTarPatch(w http.ResponseWriter, r *http.Request, basePath string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var report bytes.Buffer
	tr := tar.NewReader(bytes.NewReader(body))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		entryPath := resolveTarEntryPath(basePath, hdr.Name)
		if hdr.PAXRecords != nil {
			if _, ok := hdr.PAXRecords["delete"]; ok {
				if err := fs.RemoveAll(s.fs, entryPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				fmt.Fprintf(&report, "- %s\n", s.httpPath(entryPath))
				continue
			}
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			mode := fs.FileMode(hdr.Mode) | fs.ModeDir
			if err := fs.MkdirAll(s.fs, entryPath, mode.Perm()); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if chmodFS, ok := s.fs.(interface {
				Chmod(name string, mode fs.FileMode) error
			}); ok {
				_ = chmodFS.Chmod(entryPath, mode)
			}
		case tar.TypeSymlink:
			if err := fs.Symlink(s.fs, hdr.Linkname, entryPath); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		default:
			content, err := io.ReadAll(tr)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := s.writeFileContent(entryPath, content, fs.FileMode(hdr.Mode&0777)); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if !hdr.ModTime.IsZero() {
			if chtimesFS, ok := s.fs.(interface {
				Chtimes(name string, atime, mtime time.Time) error
			}); ok {
				_ = chtimesFS.Chtimes(entryPath, hdr.ModTime, hdr.ModTime)
			}
		}

		fmt.Fprintf(&report, "+ %s\n", s.httpPath(entryPath))
	}

	w.Header().Set("Content-Type", "application/x-tar-apply")
	w.WriteHeader(http.StatusOK)
	w.Write(report.Bytes())
}

func resolveTarEntryPath(base, entryName string) string {
	entryName = strings.Trim(entryName, "/")
	if entryName == "" || entryName == "." {
		return base
	}
	if strings.HasPrefix(entryName, "./") {
		entryName = entryName[2:]
	}
	if base == "." {
		return entryName
	}
	return filepath.Join(base, entryName)
}

func ignorableChownErr(err error) bool {
	if errors.Is(err, fs.ErrPermission) || errors.Is(err, fs.ErrNotSupported) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "operation not permitted") || strings.Contains(msg, "not supported")
}
