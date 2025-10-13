package httpfs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

// Node represents a file or directory node
type Node struct {
	// Basic file information
	name    string
	path    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
	pos     int64
	content []byte
	closed  bool
	isDirty bool
	iter    *fskit.DirIter

	// Directory-specific fields
	entries []*Node // parsed directory entries (when it's a directory)

	fs  wrappedFS
	log *slog.Logger
}

// ParseNode extracts file information from HTTP headers and creates a fileNode
func ParseNode(fsys wrappedFS, path string, headers http.Header, content []byte) (*Node, error) {
	isDir := headers.Get("Content-Type") == "application/x-directory"

	// Parse size from Content-Length header first, fall back to content length
	size := int64(len(content))
	if contentLength := headers.Get("Content-Length"); contentLength != "" {
		if parsedSize, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
			size = parsedSize
		}
	}
	if headers.Get("Content-Range") != "" {
		size = parseSizeFromContentRange(headers.Get("Content-Range"))
	}

	// Set default modes if not provided
	mode := parseMode(headers.Get("Content-Mode"))
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

	var (
		entries []*Node
		raw     []byte
		err     error
	)
	if len(content) > 0 {
		raw = content
	}
	if isDir {
		entries, err = parseDirectory(fsys, path, raw)
		if err != nil {
			return nil, err
		}
	}

	return &Node{
		fs:      fsys,
		path:    path,
		size:    size,
		content: raw,
		mode:    mode,
		modTime: parseModTime(headers.Get("Content-Modified")),
		isDir:   isDir,
		entries: entries,
		log:     slog.Default(), // for now
	}, nil
}

// fs.FileInfo interface implementation
func (n *Node) Name() string       { return filepath.Base(n.Path()) }
func (n *Node) Size() int64        { return n.size }
func (n *Node) Mode() fs.FileMode  { return n.mode }
func (n *Node) ModTime() time.Time { return n.modTime }
func (n *Node) IsDir() bool        { return n.isDir }
func (n *Node) Sys() interface{}   { return nil }

// fs.DirEntry interface implementation
func (n *Node) Type() fs.FileMode {
	return n.mode.Type()
}

func (n *Node) Info() (fs.FileInfo, error) {
	// For directories, return self since we implement fs.FileInfo
	// For files, also return self
	return n, nil
}

// fs.File interface implementation (minimal)
func (n *Node) Stat() (fs.FileInfo, error) {
	if n.closed {
		return nil, fs.ErrClosed
	}
	return fs.Stat(n.fs, n.path)
}

func (n *Node) Read(p []byte) (int, error) {
	if n.closed {
		return 0, fs.ErrClosed
	}
	if n.isDir {
		return 0, fs.ErrInvalid
	}

	if n.pos >= int64(len(n.content)) {
		return 0, io.EOF
	}

	c := copy(p, n.content[n.pos:])
	n.pos += int64(c)
	return c, nil
}

func (f *Node) Write(p []byte) (int, error) {
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
	f.size = int64(len(f.content))
	f.modTime = time.Now()

	return n, nil
}

func (n *Node) Seek(offset int64, whence int) (int64, error) {
	if n.closed {
		return 0, fs.ErrClosed
	}

	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = n.pos + offset
	case io.SeekEnd:
		newPos = int64(len(n.content)) + offset
	default:
		return 0, fmt.Errorf("invalid whence")
	}

	if newPos < 0 {
		return 0, fmt.Errorf("negative position")
	}

	n.pos = newPos
	return newPos, nil
}

func (n *Node) Close() error {
	// if n.closed {
	// 	return nil
	// }
	n.closed = true
	// Reset iterator so it can be recreated fresh if the node is reopened
	n.iter = nil

	if n.isDirty && !n.isDir {
		// Fetch current metadata to preserve any Chmod/Chtimes changes
		info, err := fs.Stat(n.fs, n.path)
		if err == nil {
			// Use current server metadata (preserves Chmod/Chtimes)
			return n.fs.unwrap().WriteFileContext(context.Background(), n.path, n.content, info.Mode(), info.ModTime())
		}
		// Fallback for new files or if stat fails
		return n.fs.unwrap().WriteFileContext(context.Background(), n.path, n.content, n.mode, n.modTime)
	}
	return nil
}

// fs.ReadDirFile interface implementation
func (n *Node) ReadDir(count int) ([]fs.DirEntry, error) {
	n.log.Debug("FileReadDir", "path", n.Path(), "count", count)
	// if n.closed {
	// 	return nil, fs.ErrClosed
	// }
	if !n.isDir {
		return nil, fs.ErrInvalid
	}
	if n.iter == nil {
		n.iter = fskit.NewDirIter(func() (entries []fs.DirEntry, err error) {
			// Fetch entries once when iterator is created
			// This ensures consistent pagination through the same snapshot
			// Use Path() to get normalized path without leading slash
			return fs.ReadDir(n.fs, n.Path())
		})
	}
	return n.iter.ReadDir(count)
}

// Path returns the full path of this node
func (n *Node) Path() string {
	if n.path == "/" {
		return "."
	}
	return strings.TrimPrefix(n.path, "/")
}

// Entries returns the directory entries (for directories)
func (n *Node) Entries() []fs.DirEntry {
	entries := make([]fs.DirEntry, len(n.entries))
	for i, entry := range n.entries {
		entries[i] = entry
	}
	return entries
}
