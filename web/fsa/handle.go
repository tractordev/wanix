//go:build js && wasm

package fsa

import (
	"io"
	"sync"
	"syscall/js"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/web/jsutil"
)

var (
	DefaultFileMode = fs.FileMode(0744)
	DefaultDirMode  = fs.FileMode(0755)
	CacheDuration   = time.Millisecond * 100
)

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
	v, cached := statCache.Load(h.name)
	if cached && v.(Stat).Name != "" && time.Since(v.(Stat).Atime) < CacheDuration {
		fi := v.(Stat).Info()
		return fi, nil
	}
	if err := h.tryGetFile(); err != nil {
		return nil, err
	}
	isDir := h.Value.Get("kind").String() == "directory"
	modTime := h.file.Get("lastModified").Int()
	var mode fs.FileMode
	if cached {
		mode = v.(Stat).Mode
	}
	if isDir {
		if mode&0777 == 0 {
			mode |= DefaultDirMode
		}
		mode |= fs.ModeDir
	} else {
		if mode&0777 == 0 {
			mode |= DefaultFileMode
		}
	}
	s := Stat{
		Name:  h.Name(),
		Size:  uint64(h.Size()),
		Mode:  mode,
		Mtime: time.UnixMilli(int64(modTime)),
		Atime: time.Now(),
	}
	statCache.Store(h.name, s) // todo: replace with statStore
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
