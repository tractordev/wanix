package httpfs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/pstat"
)

// Server implements an HTTP server that serves an fs.FS using the httpfs protocol
type Server struct {
	fs     fs.FS
	prefix string
}

// NewServer creates a new HTTP server for the given filesystem
func NewServer(fsys fs.FS) *Server {
	return &Server{
		fs:     fsys,
		prefix: "",
	}
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
		s.handleMove(w, r, path)
	default:
		w.Header().Set("Allow", "GET, HEAD, PUT, DELETE, PATCH, MOVE")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
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
		// Directory metadata streaming
		path = strings.TrimSuffix(path, "/:")
		s.handleDirStream(w, r, path)
		return
	}

	// Check if Accept header requests multipart
	if r.Header.Get("Accept") == "multipart/mixed" {
		info, err := fs.Stat(s.fs, path)
		if err != nil {
			if err == fs.ErrNotExist {
				http.Error(w, "Not Found", http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		if info.IsDir() {
			s.handleDirStream(w, r, path)
			return
		}
	}

	// Normal GET request - use WithNoFollow to get symlink metadata, not target
	ctx := fs.WithNoFollow(r.Context())
	info, err := fs.StatContext(ctx, s.fs, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "Not Found", http.StatusNotFound)
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
		// Return directory listing
		entries, err := fs.ReadDir(s.fs, path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var buf bytes.Buffer
		for _, entry := range entries {
			mode := entry.Type()
			if info, err := entry.Info(); err == nil {
				mode = info.Mode()
			}
			unixMode := pstat.FileModeToUnixMode(mode)
			fmt.Fprintf(&buf, "%s %d\n", entry.Name(), unixMode)
		}

		w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
	} else {
		// Return file content
		fsOpener, ok := s.fs.(interface {
			Open(name string) (fs.File, error)
		})
		if !ok {
			http.Error(w, "Open not supported", http.StatusInternalServerError)
			return
		}

		file, err := fsOpener.Open(path)
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
			http.Error(w, "Not Found", http.StatusNotFound)
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

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}

	if isDir {
		// Create directory
		mkdirFS, ok := s.fs.(interface {
			Mkdir(name string, perm fs.FileMode) error
		})
		if !ok {
			http.Error(w, "Directories not supported", http.StatusNotImplemented)
			return
		}

		mode := s.parseModeHeader(r.Header.Get("Content-Mode"), 0755|fs.ModeDir)
		if err := mkdirFS.Mkdir(path, mode); err != nil {
			// Check if directory already exists
			if info, statErr := fs.Stat(s.fs, path); statErr == nil && info.IsDir() {
				// Directory exists, update metadata if needed
				s.updateMetadata(w, r, path)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Update metadata if provided
		if r.Header.Get("Content-Mode") != "" || r.Header.Get("Content-Ownership") != "" || r.Header.Get("Content-Modified") != "" {
			s.updateMetadata(w, r, path)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}

	// Read content first
	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// For memfs and similar, we need to set the content directly on the node
	// Check if we can get access to raw node operations
	if ns, ok := s.fs.(interface {
		SetNode(name string, node *fskit.Node)
	}); ok {
		// Use fskit to create a proper node with content
		mode := s.parseModeHeader(r.Header.Get("Content-Mode"), 0644)
		node := createFileNode(path, content, mode)
		ns.SetNode(path, node)
	} else {
		// Fallback to Create + Write pattern
		createFS, ok := s.fs.(interface {
			Create(name string) (fs.File, error)
		})
		if !ok {
			http.Error(w, "File creation not supported", http.StatusNotImplemented)
			return
		}

		file, err := createFS.Create(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Try to write content
		if writeFile, ok := file.(interface {
			Write([]byte) (int, error)
		}); ok {
			if _, err := writeFile.Write(content); err != nil {
				file.Close()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		file.Close()
	}

	// Update metadata if provided
	if r.Header.Get("Content-Mode") != "" || r.Header.Get("Content-Ownership") != "" || r.Header.Get("Content-Modified") != "" {
		s.updateMetadata(w, r, path)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleDelete handles DELETE requests
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, path string) {
	removeFS, ok := s.fs.(interface {
		Remove(name string) error
	})
	if !ok {
		http.Error(w, "Remove not supported", http.StatusNotImplemented)
		return
	}

	if err := removeFS.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handlePatch handles PATCH requests for metadata updates
func (s *Server) handlePatch(w http.ResponseWriter, r *http.Request, path string) {
	s.updateMetadata(w, r, path)
}

// handleMove handles MOVE requests
func (s *Server) handleMove(w http.ResponseWriter, r *http.Request, path string) {
	dest := r.Header.Get("Destination")
	if dest == "" {
		http.Error(w, "Destination header required", http.StatusBadRequest)
		return
	}

	// Strip leading slash from destination
	dest = strings.TrimPrefix(dest, "/")

	renameFS, ok := s.fs.(interface {
		Rename(oldname, newname string) error
	})
	if !ok {
		http.Error(w, "Rename not supported", http.StatusNotImplemented)
		return
	}

	if err := renameFS.Rename(path, dest); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleDirStream handles directory metadata streaming with multipart response
func (s *Server) handleDirStream(w http.ResponseWriter, r *http.Request, path string) {
	info, err := fs.Stat(s.fs, path)
	if err != nil {
		if err == fs.ErrNotExist {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if !info.IsDir() {
		http.Error(w, "Not a directory", http.StatusBadRequest)
		return
	}

	entries, err := fs.ReadDir(s.fs, path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create multipart writer
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// First part: the directory itself
	part, err := mw.CreatePart(s.createPartHeaders(info, path, true))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write directory listing
	var dirContent bytes.Buffer
	for _, entry := range entries {
		mode := entry.Type()
		if entryInfo, err := entry.Info(); err == nil {
			mode = entryInfo.Mode()
		}
		unixMode := pstat.FileModeToUnixMode(mode)
		fmt.Fprintf(&dirContent, "%s %d\n", entry.Name(), unixMode)
	}
	part.Write(dirContent.Bytes())

	// Add parts for each entry
	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}

		part, err := mw.CreatePart(s.createPartHeaders(entryInfo, entryPath, entryInfo.IsDir()))
		if err != nil {
			continue
		}

		if entryInfo.IsDir() {
			// For directories, include the directory listing
			subEntries, err := fs.ReadDir(s.fs, entryPath)
			if err == nil {
				var subDirContent bytes.Buffer
				for _, subEntry := range subEntries {
					subMode := subEntry.Type()
					if subInfo, err := subEntry.Info(); err == nil {
						subMode = subInfo.Mode()
					}
					unixMode := pstat.FileModeToUnixMode(subMode)
					fmt.Fprintf(&subDirContent, "%s %d\n", subEntry.Name(), unixMode)
				}
				part.Write(subDirContent.Bytes())
			}
		}
		// For files, no body (metadata only)
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
		if err == fs.ErrNotExist {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if !info.IsDir() {
		http.Error(w, "Not a directory", http.StatusBadRequest)
		return
	}

	// Create multipart writer
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Recursively walk the directory tree
	var walkDir func(string) error
	walkDir = func(dirPath string) error {
		dirInfo, err := fs.Stat(s.fs, dirPath)
		if err != nil {
			return err
		}

		entries, err := fs.ReadDir(s.fs, dirPath)
		if err != nil {
			return err
		}

		// Add part for this directory
		part, err := mw.CreatePart(s.createPartHeaders(dirInfo, dirPath, true))
		if err != nil {
			return err
		}

		// Write directory listing
		var dirContent bytes.Buffer
		for _, entry := range entries {
			mode := entry.Type()
			if entryInfo, err := entry.Info(); err == nil {
				mode = entryInfo.Mode()
			}
			unixMode := pstat.FileModeToUnixMode(mode)
			fmt.Fprintf(&dirContent, "%s %d\n", entry.Name(), unixMode)
		}
		part.Write(dirContent.Bytes())

		// Recursively process subdirectories
		for _, entry := range entries {
			if entry.IsDir() {
				subPath := filepath.Join(dirPath, entry.Name())
				if err := walkDir(subPath); err != nil {
					continue
				}
			}
		}

		return nil
	}

	if err := walkDir(path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	mw.Close()

	w.Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

// writeMetadataHeaders writes filesystem metadata as HTTP headers
func (s *Server) writeMetadataHeaders(w http.ResponseWriter, info fs.FileInfo, path string) {
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

// createPartHeaders creates HTTP headers for a multipart part
func (s *Server) createPartHeaders(info fs.FileInfo, path string, includeBody bool) map[string][]string {
	headers := make(map[string][]string)

	// Content-Location (preferred) or Content-Disposition (backward compat)
	headers["Content-Location"] = []string{"/" + path}

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

	// Size information
	if includeBody {
		// For directories with body, use Content-Length
		headers["Content-Length"] = []string{strconv.FormatInt(info.Size(), 10)}
	} else {
		// For files without body, use Content-Range
		headers["Content-Range"] = []string{fmt.Sprintf("bytes 0-0/%d", info.Size())}
	}

	return headers
}

// updateMetadata updates file/directory metadata
func (s *Server) updateMetadata(w http.ResponseWriter, r *http.Request, path string) {
	// Check if file exists
	if _, err := fs.Stat(s.fs, path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "Not Found", http.StatusNotFound)
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
			if err := chownFS.Chown(path, uid, gid); err != nil {
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

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
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
