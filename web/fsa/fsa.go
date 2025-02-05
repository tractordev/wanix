//go:build js && wasm

// File System Access API
package fsa

import (
	"cmp"
	"context"
	"io"
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

func (fsys FS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

func (fsys FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	if name == "." {
		return DirHandleFile(js.Value(fsys)), nil
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return nil, fs.ErrNotExist
	}

	file, err := jsutil.AwaitErr(dirHandle.Call("getFileHandle", path.Base(name), map[string]any{"create": false}))
	if err == nil {
		return NewFileHandle(file, true), nil
	}

	dir, err := jsutil.AwaitErr(dirHandle.Call("getDirectoryHandle", path.Base(name), map[string]any{"create": false}))
	if err == nil {
		return DirHandleFile(dir), nil
	}

	return nil, fs.ErrNotExist
}

func (fsys FS) Create(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return nil, err
	}
	if ok {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrExist}
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return nil, fs.ErrNotExist
	}

	handle, err := jsutil.AwaitErr(dirHandle.Call("getFileHandle", path.Base(name), map[string]any{"create": true}))
	if err != nil {
		return nil, err
	}
	return NewFileHandle(handle, false), nil
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
	return err
}

func (fsys FS) walkDir(path string) (js.Value, error) {
	if path == "." {
		return js.Value(fsys), nil
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
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

func DirHandleFile(v js.Value) fs.File {
	var entries []fs.DirEntry
	jsutil.AsyncIter(v.Call("values"), func(e js.Value) error {
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
	name := cmp.Or(v.Get("name").String(), ".")
	return fskit.DirFile(fskit.Entry(name, 0755|fs.ModeDir), entries...)
}

type FileHandle struct {
	file   js.Value
	writer js.Value
	sync   js.Value
	offset int64
	closed bool
	mu     sync.Mutex
	js.Value
}

func NewFileHandle(v js.Value, append bool) *FileHandle {
	h := &FileHandle{Value: v}
	fileProm := v.Call("getFile")
	writerProm := v.Call("createWritable", map[string]any{"keepExistingData": append})

	hasSync := !v.Get("createSyncAccessHandle").IsUndefined()
	if hasSync {
		h.sync, _ = jsutil.AwaitErr(v.Call("createSyncAccessHandle"))
	}

	h.file, _ = jsutil.AwaitErr(fileProm)
	h.writer, _ = jsutil.AwaitErr(writerProm)

	return h
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

func (h *FileHandle) Size() int64 {
	if !h.sync.IsUndefined() {
		return int64(h.sync.Call("getSize").Int())
	}
	return int64(h.file.Get("size").Int())
}

func (h *FileHandle) Stat() (fs.FileInfo, error) {
	isDir := h.Value.Get("kind").String() == "directory"
	modTime := h.file.Get("lastModified").Int()
	var mode fs.FileMode
	if isDir {
		mode = 0755 | fs.ModeDir
	} else {
		mode = 0644
	}
	return fskit.Entry(h.Name(), mode, h.Size(), time.UnixMilli(int64(modTime))), nil
}

func (h *FileHandle) Write(b []byte) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return 0, fs.ErrClosed
	}

	if h.writer.IsUndefined() {
		return 0, &fs.PathError{Op: "write", Path: h.Name(), Err: fs.ErrPermission}
	}

	jsbuf := js.Global().Get("Uint8Array").New(len(b))
	n := js.CopyBytesToJS(jsbuf, b)

	_, err := jsutil.AwaitErr(h.writer.Call("write", jsbuf))
	if err != nil {
		return 0, err
	}

	return n, nil
}

func (h *FileHandle) Read(b []byte) (int, error) {
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
