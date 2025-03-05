//go:build js && wasm

// File System Access API
package fsa

import (
	"cmp"
	"context"
	"io"
	"log"
	"path"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/web/jsutil"
)

// user selected directory
func ShowDirectoryPicker() fs.FS {
	dir := jsutil.Await(js.Global().Get("window").Call("showDirectoryPicker"))
	return FS(dir)
}

// origin private file system
func OPFS() (fs.FS, error) {
	dir, err := jsutil.AwaitErr(js.Global().Get("navigator").Get("storage").Call("getDirectory"))
	if err != nil {
		return nil, err
	}
	return FS(dir), nil
}

type FS js.Value

type stat struct {
	name  string
	size  uint64
	mode  fs.FileMode
	atime time.Time
	mtime time.Time
}

func (s stat) Info() fs.FileInfo {
	return fskit.Entry(path.Base(s.name), s.mode, s.size, s.mtime)
}

var statCache sync.Map

func (fsys FS) walkDir(path string) (js.Value, error) {
	if path == "." {
		return js.Value(fsys), nil
	}
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	cur := js.Value(fsys)
	var err error
	for i := 0; i < len(parts); i++ {
		cur, err = jsutil.AwaitErr(cur.Call("getDirectoryHandle", parts[i], map[string]any{"create": false}))
		if err != nil {
			return js.Undefined(), err
		}
	}
	return cur, nil
}

// Chtimes is not supported but implement as no-op
func (fsys FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chtimes", Path: name, Err: fs.ErrInvalid}
	}

	v, ok := statCache.Load(name)
	if ok {
		stat := v.(stat)
		// stat.atime = atime
		stat.mtime = mtime
		statCache.Store(name, stat)
		return nil
	}
	statCache.Store(name, stat{atime: atime, mtime: mtime})

	return nil
}

func (fsys FS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

func (fsys FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	if name == "." {
		return DirHandleFile(fsys, name, js.Value(fsys)), nil
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return nil, fs.ErrNotExist
	}

	file, err := jsutil.AwaitErr(dirHandle.Call("getFileHandle", path.Base(name), map[string]any{"create": false}))
	if err == nil {
		return NewFileHandle(name, file, true), nil
	}

	dir, err := jsutil.AwaitErr(dirHandle.Call("getDirectoryHandle", path.Base(name), map[string]any{"create": false}))
	if err == nil {
		return DirHandleFile(fsys, name, dir), nil
	}

	return nil, fs.ErrNotExist
}

func (fsys FS) Truncate(name string, size int64) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "truncate", Path: name, Err: fs.ErrNotExist}
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return fs.ErrNotExist
	}

	file, err := jsutil.AwaitErr(dirHandle.Call("getFileHandle", path.Base(name), map[string]any{"create": false}))
	if err == nil {
		return NewFileHandle(name, file, false).Truncate(size)
	}

	return fs.ErrNotExist
}

func (fsys FS) Create(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrNotExist}
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return nil, fs.ErrNotExist
	}

	handle, err := jsutil.AwaitErr(dirHandle.Call("getFileHandle", path.Base(name), map[string]any{"create": true}))
	if err != nil {
		return nil, err
	}
	return NewFileHandle(name, handle, false), nil
}

func (fsys FS) Mkdir(name string, perm fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if ok {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrExist}
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
	}

	_, err = jsutil.AwaitErr(dirHandle.Call("getDirectoryHandle", path.Base(name), map[string]any{"create": true}))
	return err
}

func (fsys FS) Remove(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	if isDir, err := fs.IsDir(fsys, name); err != nil {
		return err
	} else if isDir {
		empty, err := fs.IsEmpty(fsys, name)
		if err != nil {
			return err
		}
		if !empty {
			return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotEmpty}
		}
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	_, err = jsutil.AwaitErr(dirHandle.Call("removeEntry", path.Base(name)))
	if err != nil {
		return err
	}
	statCache.Delete(name)
	return nil
}

func DirHandleFile(fsys FS, name string, v js.Value) fs.File {
	var entries []fs.DirEntry
	err := jsutil.AsyncIter(v.Call("values"), func(e js.Value) error {
		name := e.Get("name").String()
		var mode fs.FileMode
		var size int64
		if e.Get("kind").String() == "directory" {
			mode = 0755 | fs.ModeDir
		} else {
			mode = 0644
			size = int64(jsutil.Await(e.Call("getFile")).Get("size").Int())
		}
		entries = append(entries, fskit.Entry(name, mode, size))
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	fname := cmp.Or(v.Get("name").String(), ".")
	return fskit.DirFile(fskit.Entry(fname, 0755|fs.ModeDir), entries...)
}

type FileHandle struct {
	name   string
	append bool
	file   js.Value
	writer js.Value
	sync   js.Value
	offset int64
	closed bool
	mu     sync.Mutex
	js.Value
}

func NewFileHandle(name string, v js.Value, append bool) *FileHandle {
	h := &FileHandle{Value: v, name: name, append: append}

	// hasSync := !v.Get("createSyncAccessHandle").IsUndefined()
	// if hasSync {
	// 	h.sync, _ = jsutil.AwaitErr(v.Call("createSyncAccessHandle"))
	// }

	return h
}

func (h *FileHandle) tryGetFile() (err error) {
	if !h.file.IsUndefined() {
		return nil
	}
	h.file, err = jsutil.AwaitErr(h.Value.Call("getFile"))
	return
}

func (h *FileHandle) tryCreateWritable() (err error) {
	if !h.writer.IsUndefined() {
		return nil
	}
	if h.sync.IsUndefined() {
		h.writer, err = jsutil.AwaitErr(h.Value.Call("createWritable", map[string]any{"keepExistingData": h.append}))
	}
	return
}

func (h *FileHandle) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return fs.ErrClosed
	}

	h.closed = true

	if !h.sync.IsUndefined() {
		h.sync.Call("close")
	}

	if !h.writer.IsUndefined() {
		_, err := jsutil.AwaitErr(h.writer.Call("close"))
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *FileHandle) Name() string {
	return h.Value.Get("name").String()
}

func (h *FileHandle) Truncate(size int64) error {
	if err := h.tryCreateWritable(); err != nil {
		return err
	}
	// if !h.sync.IsUndefined() {
	// 	h.sync.Call("truncate", size)
	// 	return nil
	// }
	if !h.writer.IsUndefined() {
		jsutil.Await(h.writer.Call("write", map[string]any{
			"type": "truncate",
			"size": size,
		}))
		return nil
	}
	return fs.ErrPermission
}

func (h *FileHandle) Size() int64 {
	if !h.sync.IsUndefined() {
		return int64(h.sync.Call("getSize").Int())
	}
	h.tryGetFile()
	return int64(h.file.Get("size").Int())
}

func (h *FileHandle) Stat() (fs.FileInfo, error) {
	v, ok := statCache.Load(h.name)
	if ok && time.Since(v.(stat).atime) < time.Second {
		return v.(stat).Info(), nil
	}
	if err := h.tryGetFile(); err != nil {
		return nil, err
	}
	isDir := h.Value.Get("kind").String() == "directory"
	modTime := h.file.Get("lastModified").Int()
	var mode fs.FileMode
	if isDir {
		mode = 0755 | fs.ModeDir
	} else {
		mode = 0744
	}
	s := stat{
		name:  h.Name(),
		size:  uint64(h.Size()),
		mode:  mode,
		mtime: time.UnixMilli(int64(modTime)),
		atime: time.Now(),
	}
	statCache.Store(h.name, s)
	return s.Info(), nil
}

func (h *FileHandle) Write(b []byte) (int, error) {
	if err := h.tryCreateWritable(); err != nil {
		return 0, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return 0, fs.ErrClosed
	}

	if h.sync.IsUndefined() && h.writer.IsUndefined() {
		return 0, &fs.PathError{Op: "write", Path: h.Name(), Err: fs.ErrPermission}
	}

	jsbuf := js.Global().Get("Uint8Array").New(len(b))
	n := js.CopyBytesToJS(jsbuf, b)

	if h.sync.IsUndefined() {
		_, err := jsutil.AwaitErr(h.writer.Call("write", map[string]any{
			"type":     "write",
			"data":     jsbuf,
			"position": h.offset,
		}))
		if err != nil {
			return 0, err
		}
	} else {
		nn := h.sync.Call("write", jsbuf, map[string]any{
			"at": h.offset,
		})
		n = int(nn.Int())
	}
	h.offset += int64(n)

	return n, nil
}

func (h *FileHandle) Read(b []byte) (int, error) {
	if err := h.tryGetFile(); err != nil {
		return 0, err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return 0, fs.ErrClosed
	}

	size := h.Size()
	if h.offset >= size {
		return 0, io.EOF
	}
	if h.offset < 0 {
		return 0, &fs.PathError{Op: "read", Path: h.Name(), Err: fs.ErrInvalid}
	}
	rest := int(size - h.offset)
	if len(b) < rest {
		rest = len(b)
	}
	var n int
	if !h.sync.IsUndefined() {
		jsbuf := js.Global().Get("Uint8Array").New(rest)
		h.sync.Call("read", jsbuf, map[string]any{
			"at": h.offset,
		})
		n = js.CopyBytesToGo(b, jsbuf)
	} else {
		restblob := h.file.Call("slice", h.offset)
		arrbuf := jsutil.Await(restblob.Call("arrayBuffer"))
		jsbuf := js.Global().Get("Uint8Array").New(arrbuf)
		n = js.CopyBytesToGo(b, jsbuf)
	}
	h.offset += int64(n)
	return n, nil
}

func (h *FileHandle) Seek(offset int64, whence int) (int64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return 0, fs.ErrClosed
	}

	end := h.Size()
	if h.offset > end {
		end = h.offset
	}
	switch whence {
	case 0:
		// offset += 0
	case 1:
		offset += h.offset
	case 2:
		offset += end
	}
	if offset > end {
		offset = end
	}
	if offset < 0 {
		return 0, &fs.PathError{Op: "seek", Path: h.Name(), Err: fs.ErrInvalid}
	}
	h.offset = offset
	return offset, nil
}
