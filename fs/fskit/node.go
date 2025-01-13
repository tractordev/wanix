package fskit

import (
	"context"
	"io"
	"path"
	"time"

	"tractor.dev/wanix/fs"
)

// N is used to create an fs.FileInfo, fs.DirEntry, or fs.FS.
type N struct {
	name    string
	mode    fs.FileMode
	modTime time.Time
	size    int64
	sys     any
	data    []byte
	// nodes   []*N
}

func Node(attrs ...any) *N {
	n := &N{}
	for _, m := range attrs {
		switch v := m.(type) {
		case int:
			n.size = int64(v)
		case time.Time:
			n.modTime = v
		case []byte:
			n.data = v
		case string:
			n.name = v
		case fs.FileMode:
			n.mode = v
		case fs.FileInfo:
			n.name = v.Name()
			n.mode = v.Mode()
			n.size = v.Size()
			n.modTime = v.ModTime()
			n.sys = v.Sys()
			// case *N:
			// n.nodes = append(n.nodes, v)
		}
	}
	return n
}

func Entry(name string, mode fs.FileMode, more ...any) *N {
	n := Node(more...)
	n.name = name
	n.mode = mode
	return n
}

// fs.FileInfo and fs.DirEntry interfaces implemented
var _ = (fs.FileInfo)((*N)(nil))
var _ = (fs.DirEntry)((*N)(nil))

func (n *N) Name() string               { return path.Base(n.name) }
func (n *N) Info() (fs.FileInfo, error) { return n, nil }
func (n *N) Mode() fs.FileMode          { return n.mode }
func (n *N) Type() fs.FileMode          { return n.mode.Type() }
func (n *N) ModTime() time.Time         { return n.modTime }
func (n *N) IsDir() bool                { return n.mode&fs.ModeDir != 0 }
func (n *N) Sys() any                   { return n.sys }

func (n *N) Size() int64 {
	if n.size > 0 {
		return n.size
	}
	return int64(len(n.data))
}

func (n *N) String() string {
	return fs.FormatFileInfo(n)
}

func SetName(fsys fs.FS, name string) {
	n, ok := fsys.(*N)
	if !ok {
		panic("not a *N")
	}
	n.name = name
}

// fs.OpenContextFS
var _ = (fs.OpenContextFS)((*N)(nil))

func (n *N) Open(name string) (fs.File, error) {
	return n.OpenContext(context.Background(), name)
}

// TODO: open sub nodes
func (n *N) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if name == "." {
		return &nodeFile{N: n}, nil
	}
	return nil, fs.ErrNotExist
}

type nodeFile struct {
	*N
	offset int64
}

func (f *nodeFile) Close() error { return nil }

func (f *nodeFile) Stat() (fs.FileInfo, error) {
	return f, nil
}

func (f *nodeFile) Read(b []byte) (int, error) {
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
	if offset < 0 || offset > int64(len(f.data)) {
		return 0, &fs.PathError{Op: "read", Path: f.name, Err: fs.ErrInvalid}
	}
	n := copy(b, f.data[offset:])
	if n < len(b) {
		return n, io.EOF
	}
	return n, nil
}

// dirFile is a directory fs.File implementing fs.ReadDirFile
type dirFile struct {
	fs.FileInfo
	path    string
	entries []fs.DirEntry
	offset  int
}

func DirFile(info *N, entries ...fs.DirEntry) fs.File {
	if !info.IsDir() {
		info.mode |= fs.ModeDir
	}
	return &dirFile{FileInfo: info, path: info.name, entries: entries}
}

func (d *dirFile) Stat() (fs.FileInfo, error) { return d, nil }
func (d *dirFile) Close() error               { return nil }
func (d *dirFile) Read(b []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.path, Err: fs.ErrInvalid}
}

func (d *dirFile) ReadDir(count int) ([]fs.DirEntry, error) {
	n := len(d.entries) - d.offset
	if n == 0 && count > 0 {
		return nil, io.EOF
	}
	if count > 0 && n > count {
		n = count
	}
	list := make([]fs.DirEntry, n)
	for i := range list {
		list[i] = d.entries[d.offset+i]
	}
	d.offset += n
	return list, nil
}
