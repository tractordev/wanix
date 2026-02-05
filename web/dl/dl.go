//go:build js && wasm

package dl

import (
	"io"
	"os"
	"path"
	"sync"
	"syscall/js"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

func Download(data []byte, filename string) error {
	buf := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(buf, data)
	blob := js.Global().Get("Blob").New([]any{buf}, js.ValueOf(map[string]any{"type": "application/octet-stream"}))
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	doc := js.Global().Get("document")
	a := doc.Call("createElement", "a")
	a.Set("href", url)
	a.Set("download", filename)
	a.Call("click")
	js.Global().Get("URL").Call("revokeObjectURL", url)
	return nil
}

// FS is a virtual filesystem that triggers browser downloads on file close.
// Files written to this filesystem don't persist - instead, when closed,
// their contents are downloaded in the browser with the filename.
type FS struct {
	mu    sync.Mutex
	files map[string]*dlFile
}

// New creates a new download filesystem.
func New() *FS {
	return &FS{
		files: make(map[string]*dlFile),
	}
}

var (
	_ fs.FS         = (*FS)(nil)
	_ fs.CreateFS   = (*FS)(nil)
	_ fs.OpenFileFS = (*FS)(nil)
)

func (fsys *FS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	name = path.Clean(name)

	if name == "." {
		// Return root directory
		return fskit.DirFile(fskit.Entry(".", fs.ModeDir|0755, time.Now())), nil
	}

	// Check if there's an open file being written
	fsys.mu.Lock()
	f, exists := fsys.files[name]
	fsys.mu.Unlock()

	if exists {
		// Return a read view of the file being written
		f.mu.Lock()
		dataCopy := make([]byte, len(f.data))
		copy(dataCopy, f.data)
		f.mu.Unlock()
		return &dlReadFile{
			name: name,
			data: dataCopy,
		}, nil
	}

	// Files don't persist, so return not found
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}

	name = path.Clean(name)

	if name == "." {
		return fskit.Entry(".", fs.ModeDir|0755, time.Now()), nil
	}

	// Files don't persist
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

func (fsys *FS) Create(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrInvalid}
	}

	name = path.Clean(name)

	if name == "." {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrInvalid}
	}

	f := &dlFile{
		fsys: fsys,
		name: name,
		data: nil,
	}

	fsys.mu.Lock()
	fsys.files[name] = f
	fsys.mu.Unlock()

	return f, nil
}

func (fsys *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	name = path.Clean(name)

	// Root directory
	if name == "." {
		return fskit.DirFile(fskit.Entry(".", fs.ModeDir|0755, time.Now())), nil
	}

	// For any write operation, create a new download file
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE) != 0 {
		return fsys.Create(name)
	}

	// Read-only: check if file is currently being written
	fsys.mu.Lock()
	f, exists := fsys.files[name]
	fsys.mu.Unlock()

	if exists {
		f.mu.Lock()
		dataCopy := make([]byte, len(f.data))
		copy(dataCopy, f.data)
		f.mu.Unlock()
		return &dlReadFile{
			name: name,
			data: dataCopy,
		}, nil
	}

	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// dlFile is a file that buffers writes and triggers a download on close.
type dlFile struct {
	fsys   *FS
	name   string
	data   []byte
	offset int64
	closed bool
	mu     sync.Mutex
}

func (f *dlFile) Stat() (fs.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fskit.Entry(f.name, 0644, time.Now(), int64(len(f.data))), nil
}

func (f *dlFile) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, fs.ErrClosed
	}
	if f.offset >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n := copy(p, f.data[f.offset:])
	f.offset += int64(n)
	return n, nil
}

func (f *dlFile) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, fs.ErrClosed
	}

	n := len(p)
	endPos := f.offset + int64(n)

	// Grow data slice if needed
	if endPos > int64(len(f.data)) {
		newData := make([]byte, endPos)
		copy(newData, f.data)
		f.data = newData
	}

	copy(f.data[f.offset:], p)
	f.offset += int64(n)
	return n, nil
}

func (f *dlFile) Seek(offset int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, fs.ErrClosed
	}

	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = f.offset + offset
	case io.SeekEnd:
		newOffset = int64(len(f.data)) + offset
	default:
		return 0, &fs.PathError{Op: "seek", Path: f.name, Err: fs.ErrInvalid}
	}

	if newOffset < 0 {
		return 0, &fs.PathError{Op: "seek", Path: f.name, Err: fs.ErrInvalid}
	}

	f.offset = newOffset
	return newOffset, nil
}

func (f *dlFile) WriteAt(p []byte, off int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, fs.ErrClosed
	}
	if off < 0 {
		return 0, &fs.PathError{Op: "writeat", Path: f.name, Err: fs.ErrInvalid}
	}

	n := len(p)
	endPos := off + int64(n)

	// Grow data slice if needed
	if endPos > int64(len(f.data)) {
		newData := make([]byte, endPos)
		copy(newData, f.data)
		f.data = newData
	}

	copy(f.data[off:], p)
	return n, nil
}

func (f *dlFile) ReadAt(p []byte, off int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, fs.ErrClosed
	}
	if off < 0 || off >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n := copy(p, f.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (f *dlFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return fs.ErrClosed
	}
	f.closed = true

	// Remove from tracking
	f.fsys.mu.Lock()
	delete(f.fsys.files, f.name)
	f.fsys.mu.Unlock()

	// Trigger download with the buffered data
	if len(f.data) > 0 {
		Download(f.data, path.Base(f.name))
	}

	return nil
}

func (f *dlFile) Sync() error {
	// No-op for download files - data is only committed on close
	return nil
}

// dlReadFile is a read-only view of a file being written.
type dlReadFile struct {
	name   string
	data   []byte
	offset int
	closed bool
}

func (f *dlReadFile) Stat() (fs.FileInfo, error) {
	return fskit.Entry(f.name, 0644, time.Now(), int64(len(f.data))), nil
}

func (f *dlReadFile) Read(p []byte) (int, error) {
	if f.closed {
		return 0, fs.ErrClosed
	}
	if f.offset >= len(f.data) {
		return 0, io.EOF
	}
	n := copy(p, f.data[f.offset:])
	f.offset += n
	return n, nil
}

func (f *dlReadFile) Close() error {
	f.closed = true
	return nil
}
