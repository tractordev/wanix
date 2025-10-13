package fskit

import (
	"bytes"
	"context"
	"io"
	"path"
	"sync"
	"time"

	"tractor.dev/wanix/fs"
)

// Node is used to create an fs.FileInfo, fs.DirEntry, or fs.FS.
type Node struct {
	name    string
	mode    fs.FileMode
	modTime time.Time
	size    int64
	sys     any
	uid     int
	gid     int
	data    []byte

	reader io.Reader
	writer io.Writer

	// nodes   []*N
}

func RawNode(attrs ...any) *Node {
	n := &Node{}
	for _, m := range attrs {
		switch v := m.(type) {
		case *Node:
			n.name = v.name
			n.mode = v.mode
			n.size = v.size
			n.modTime = v.modTime
			n.sys = v.sys
			n.uid = v.uid
			n.gid = v.gid
			n.data = v.data
			n.reader = v.reader
			n.writer = v.writer
		case int64:
			n.size = v
		case int:
			n.size = int64(v)
		case uint64:
			n.size = int64(v)
		case time.Time:
			n.modTime = v
		case []byte:
			n.data = v
		case string:
			n.name = v
		case fs.FileMode:
			n.mode = v
			if v&fs.ModeDir != 0 && n.size == 0 {
				n.size = 2 // Set initial size to 2 for "." and ".." entries
			}
		case fs.FileInfo:
			n.name = v.Name()
			n.mode = v.Mode()
			n.size = v.Size()
			n.modTime = v.ModTime()
			n.sys = v.Sys()
			if n.mode&fs.ModeDir != 0 && n.size == 0 {
				n.size = 2 // Set initial size to 2 for "." and ".." entries
			}

		// these must come after fs.FileInfo since
		// some of our fs.FileInfo implementations
		// are also io.Readers...
		case io.ReadWriter:
			n.reader = v
			n.writer = v
		case io.Reader:
			n.reader = v
		case io.Writer:
			n.writer = v

			// case *N:
			// n.nodes = append(n.nodes, v)
		}
	}
	return n
}

func Entry(name string, mode fs.FileMode, more ...any) *Node {
	n := RawNode(more...)
	n.name = name
	n.mode = mode
	return n
}

// fs.FileInfo and fs.DirEntry interfaces implemented
var _ = (fs.FileInfo)((*Node)(nil))
var _ = (fs.DirEntry)((*Node)(nil))

func (n *Node) Name() string               { return path.Base(n.name) }
func (n *Node) Info() (fs.FileInfo, error) { return n, nil }
func (n *Node) Mode() fs.FileMode          { return n.mode }
func (n *Node) Type() fs.FileMode          { return n.mode.Type() }
func (n *Node) ModTime() time.Time         { return n.modTime }
func (n *Node) IsDir() bool                { return n.mode&fs.ModeDir != 0 }
func (n *Node) Sys() any                   { return n.sys }

func (n *Node) Size() int64 {
	if n.size < 0 {
		return 0
	}
	if n.size > 0 {
		return n.size
	}
	return int64(len(n.data))
}

func (n *Node) String() string {
	return fs.FormatFileInfo(n)
}

func (n *Node) Data() []byte {
	return n.data
}

func SetName(n *Node, name string) {
	n.name = name
}

func SetData(n *Node, data []byte) {
	n.data = data
}

func SetMode(n *Node, mode fs.FileMode) {
	n.mode = mode
}

func SetModTime(n *Node, modTime time.Time) {
	n.modTime = modTime
}

func SetSize(n *Node, size int64) {
	n.size = size
}

func SetSys(n *Node, sys any) {
	n.sys = sys
}

func SetUid(n *Node, uid int) {
	n.uid = uid
}

func SetGid(n *Node, gid int) {
	n.gid = gid
}

// fs.OpenContextFS
var _ = (fs.OpenContextFS)((*Node)(nil))

func (n *Node) Open(name string) (fs.File, error) {
	return n.OpenContext(context.Background(), name)
}

// TODO: open sub nodes
func (n *Node) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if name == "." {
		return n.file(), nil
	}
	return nil, fs.ErrNotExist
}

func (n *Node) file() *nodeFile {
	nn := *n
	return &nodeFile{Node: &nn, inode: n}
}

type nodeFile struct {
	*Node
	inode  *Node
	dirty  bool
	offset int64
	closed bool
	mu     sync.Mutex
}

func (f *nodeFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return fs.ErrClosed
	}

	if f.dirty && f.inode != nil {
		f.inode.data = f.data
	}

	f.closed = true
	return nil
}

func (f *nodeFile) Stat() (fs.FileInfo, error) {
	return f, nil
}

func (f *nodeFile) Read(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return 0, fs.ErrClosed
	}

	if f.reader != nil {
		return f.reader.Read(b)
	}

	if f.offset >= int64(len(f.data)) {
		return 0, io.EOF
	}
	if f.offset < 0 {
		return 0, &fs.PathError{Op: "read", Path: f.name, Err: fs.ErrInvalid}
	}
	n := copy(b, f.data[f.offset:])
	f.offset += int64(n)
	return n, nil
}

func (f *nodeFile) Seek(offset int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return 0, fs.ErrClosed
	}

	switch whence {
	case 0:
		// offset += 0
	case 1:
		offset += f.offset
	case 2:
		offset += int64(len(f.data))
	}
	if offset < 0 || offset > int64(len(f.data)) {
		return 0, &fs.PathError{Op: "seek", Path: f.name, Err: fs.ErrInvalid}
	}
	f.offset = offset
	return offset, nil
}

func (f *nodeFile) ReadAt(b []byte, offset int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return 0, fs.ErrClosed
	}

	if f.reader != nil {
		return f.reader.Read(b)
	}

	if offset < 0 || offset > int64(len(f.data)) {
		return 0, &fs.PathError{Op: "read", Path: f.name, Err: fs.ErrInvalid}
	}
	n := copy(b, f.data[offset:])
	if n < len(b) {
		return n, io.EOF
	}
	return n, nil
}

func (f *nodeFile) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return 0, fs.ErrClosed
	}

	if f.writer != nil {
		return f.writer.Write(b)
	}

	n := len(b)
	cur := f.offset
	diff := cur - int64(len(f.data))
	var tail []byte
	if n+int(cur) < len(f.data) {
		tail = f.data[n+int(cur):]
	}
	if diff > 0 {
		f.data = append(f.data, append(bytes.Repeat([]byte{00}, int(diff)), b...)...)
		f.data = append(f.data, tail...)
	} else {
		f.data = append(f.data[:cur], b...)
		f.data = append(f.data, tail...)
	}
	f.modTime = time.Now()
	f.dirty = true
	f.offset += int64(n)
	return n, nil
}

func (f *nodeFile) WriteAt(b []byte, offset int64) (int, error) {
	f.mu.Lock()

	if f.closed {
		f.mu.Unlock()
		return 0, fs.ErrClosed
	}

	if f.writer != nil {
		f.mu.Unlock()
		return f.writer.Write(b)
	}

	if offset < 0 || offset > int64(len(f.data)) {
		f.mu.Unlock()
		return 0, &fs.PathError{Op: "write", Path: f.name, Err: fs.ErrInvalid}
	}

	f.offset = offset
	f.dirty = true
	f.mu.Unlock()
	return f.Write(b)
}
