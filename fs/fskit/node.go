package fskit

import (
	"context"
	"io"
	"log/slog"
	"path"
	"sync"
	"time"

	"tractor.dev/wanix/fs"
)

// Node is used to create an fs.FileInfo, fs.DirEntry, or fs.FS.
type Node struct {
	path    string
	mode    fs.FileMode
	modTime time.Time
	size    int64
	sys     any
	uid     int
	gid     int
	data    []byte
	log     *slog.Logger

	reader io.Reader
	writer io.Writer

	mu sync.Mutex
}

func RawNode(attrs ...any) *Node {
	n := &Node{}
	for _, m := range attrs {
		switch v := m.(type) {
		case *Node:
			n.path = v.path
			n.mode = v.mode
			n.size = v.size
			n.modTime = v.modTime
			n.sys = v.sys
			n.uid = v.uid
			n.gid = v.gid
			n.data = v.data
			n.reader = v.reader
			n.writer = v.writer
			n.log = v.log
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
			n.path = v
		case fs.FileMode:
			n.mode = v
			if v&fs.ModeDir != 0 && n.size == 0 {
				n.size = 2 // Set initial size to 2 for "." and ".." entries
			}
		case fs.FileInfo:
			n.path = v.Name()
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
		case *slog.Logger:
			n.log = v

		}
	}
	if n.log == nil {
		n.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return n
}

func Entry(name string, mode fs.FileMode, more ...any) *Node {
	n := RawNode(more...)
	n.path = name
	n.mode = mode
	return n
}

// fs.FileInfo and fs.DirEntry interfaces implemented
var _ = (fs.FileInfo)((*Node)(nil))
var _ = (fs.DirEntry)((*Node)(nil))

func (n *Node) Info() (fs.FileInfo, error) { return n, nil }

func (n *Node) Path() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.path
}

func (n *Node) Name() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return path.Base(n.path)
}

func (n *Node) Mode() fs.FileMode {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.mode
}

func (n *Node) Type() fs.FileMode {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.mode.Type()
}

func (n *Node) ModTime() time.Time {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.modTime
}

func (n *Node) IsDir() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.mode&fs.ModeDir != 0
}

func (n *Node) Sys() any {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.sys
}

func (n *Node) Size() int64 {
	n.mu.Lock()
	defer n.mu.Unlock()
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
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.data == nil {
		return nil
	}
	dataCopy := make([]byte, len(n.data))
	copy(dataCopy, n.data)
	return dataCopy
}

func (n *Node) Log() *slog.Logger {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.log
}

func (n *Node) Reader() io.Reader {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.reader
}

func (n *Node) Writer() io.Writer {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.writer
}

func SetLogger(n *Node, log *slog.Logger) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.log = log
}

func SetName(n *Node, name string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.path = name
}

func SetData(n *Node, data []byte) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data = data
}

func SetMode(n *Node, mode fs.FileMode) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.mode = mode
}

func SetModTime(n *Node, modTime time.Time) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.modTime = modTime
}

func SetSize(n *Node, size int64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.size = size
}

func SetSys(n *Node, sys any) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sys = sys
}

func SetUid(n *Node, uid int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.uid = uid
}

func SetGid(n *Node, gid int) {
	n.mu.Lock()
	defer n.mu.Unlock()
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
		return n.openFile(), nil
	}
	return nil, fs.ErrNotExist
}

func (n *Node) openFile() *nodeFile {
	return &nodeFile{data: n.Data(), inode: n}
}

type nodeFile struct {
	data    []byte
	inode   *Node
	dirty   bool
	offset  int64
	closed  bool
	modTime time.Time
	mu      sync.Mutex
}

func (f *nodeFile) Close() (err error) {
	defer func() {
		f.inode.Log().Debug("close", "name", f.inode.Path(), "err", err)
	}()
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return fs.ErrClosed
	}

	if f.dirty && f.inode != nil {
		SetData(f.inode, f.data)
		SetModTime(f.inode, f.modTime)
	}

	f.closed = true
	return nil
}

func (f *nodeFile) Stat() (fs.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.inode == nil {
		return nil, fs.ErrClosed
	}
	return f.inode, nil
}

func (f *nodeFile) Read(b []byte) (n int, err error) {
	defer func() {
		f.inode.Log().Debug("read", "name", f.inode.Path(), "n", n, "err", err)
	}()

	// Check for custom reader BEFORE acquiring lock to avoid deadlock
	if reader := f.inode.Reader(); reader != nil {
		return reader.Read(b)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return 0, fs.ErrClosed
	}

	if f.offset >= int64(len(f.data)) {
		return 0, io.EOF
	}
	if f.offset < 0 {
		return 0, &fs.PathError{Op: "read", Path: f.inode.Path(), Err: fs.ErrInvalid}
	}

	// Check if offset can be represented as int (required for slice operations)
	// This prevents overflow on 32-bit systems and WASM
	const maxInt = int(^uint(0) >> 1)
	if f.offset > int64(maxInt) {
		return 0, &fs.PathError{Op: "read", Path: f.inode.Path(), Err: fs.ErrInvalid}
	}

	n = copy(b, f.data[int(f.offset):])
	f.offset += int64(n)
	return n, nil
}

func (f *nodeFile) Seek(offset int64, whence int) (newPos int64, err error) {
	defer func() {
		f.inode.Log().Debug("seek", "name", f.inode.Path(), "offset", offset, "whence", whence, "newPos", newPos, "err", err)
	}()
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
		return 0, &fs.PathError{Op: "seek", Path: f.inode.Path(), Err: fs.ErrInvalid}
	}
	f.offset = offset
	return offset, nil
}

func (f *nodeFile) ReadAt(b []byte, offset int64) (n int, err error) {
	defer func() {
		f.inode.Log().Debug("readat", "name", f.inode.Path(), "offset", offset, "n", n, "err", err)
	}()

	// Check for custom reader BEFORE acquiring lock to avoid deadlock
	if reader := f.inode.Reader(); reader != nil {
		return reader.Read(b)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return 0, fs.ErrClosed
	}

	if offset < 0 || offset > int64(len(f.data)) {
		return 0, &fs.PathError{Op: "read", Path: f.inode.Path(), Err: fs.ErrInvalid}
	}

	// Check if offset can be represented as int (required for slice operations)
	// This prevents overflow on 32-bit systems and WASM
	const maxInt = int(^uint(0) >> 1)
	if offset > int64(maxInt) {
		return 0, &fs.PathError{Op: "read", Path: f.inode.Path(), Err: fs.ErrInvalid}
	}

	n = copy(b, f.data[int(offset):])
	if n < len(b) {
		return n, io.EOF
	}
	return n, nil
}

func (f *nodeFile) Write(b []byte) (n int, err error) {
	defer func() {
		f.inode.Log().Debug("write", "name", f.inode.Path(), "n", n, "err", err)
	}()

	// Check for custom writer BEFORE acquiring lock to avoid deadlock
	if writer := f.inode.Writer(); writer != nil {
		return writer.Write(b)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return 0, fs.ErrClosed
	}

	n = len(b)
	cur := f.offset

	// Validate offset is within valid range for slice operations
	if cur < 0 {
		return 0, &fs.PathError{Op: "write", Path: f.inode.Path(), Err: fs.ErrInvalid}
	}

	// Check if offset can be represented as int (required for slice operations)
	// This prevents overflow on 32-bit systems and WASM
	const maxInt = int(^uint(0) >> 1)
	if cur > int64(maxInt) {
		return 0, &fs.PathError{Op: "write", Path: f.inode.Path(), Err: fs.ErrInvalid}
	}

	dataLen := int64(len(f.data))
	endPos := cur + int64(n)

	// Check if end position would overflow
	if endPos > int64(maxInt) {
		return 0, &fs.PathError{Op: "write", Path: f.inode.Path(), Err: fs.ErrInvalid}
	}

	curInt := int(cur)
	endPosInt := int(endPos)

	// Calculate if we need to grow the data slice
	if cur > dataLen {
		// Need to pad with zeros between current data end and write position
		// Grow data to accommodate padding + new data
		newData := make([]byte, endPosInt)
		copy(newData, f.data)
		// zeros are already in place from make()
		copy(newData[curInt:], b)
		f.data = newData
	} else if endPos > dataLen {
		// Write starts within or at the end of existing data but extends beyond
		newData := make([]byte, endPosInt)
		copy(newData, f.data)
		copy(newData[curInt:], b)
		f.data = newData
	} else {
		// Write is entirely within existing data
		copy(f.data[curInt:], b)
	}

	f.modTime = time.Now()
	f.dirty = true
	f.offset += int64(n)
	return n, nil
}

func (f *nodeFile) WriteAt(b []byte, offset int64) (n int, err error) {
	defer func() {
		f.inode.Log().Debug("writeat", "name", f.inode.Path(), "offset", offset, "n", n, "err", err)
	}()

	// Check for custom writer BEFORE acquiring lock to avoid deadlock
	if writer := f.inode.Writer(); writer != nil {
		return writer.Write(b)
	}

	f.mu.Lock()

	if f.closed {
		f.mu.Unlock()
		return 0, fs.ErrClosed
	}

	if offset < 0 || offset > int64(len(f.data)) {
		f.mu.Unlock()
		return 0, &fs.PathError{Op: "write", Path: f.inode.Path(), Err: fs.ErrInvalid}
	}

	f.offset = offset
	f.dirty = true
	f.mu.Unlock()
	return f.Write(b)
}
