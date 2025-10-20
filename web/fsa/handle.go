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
	DefaultFileMode = fs.FileMode(0644)
	DefaultDirMode  = fs.FileMode(0755)
	StatCacheExpiry = time.Millisecond * 100
)

type FileHandle struct {
	path   string // Full path for stat cache and error reporting
	append bool
	file   js.Value
	writer js.Value
	offset int64
	closed bool
	mu     sync.Mutex
	fsys   *FS // Reference to the FS instance for stat cache access
	js.Value
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
	if h.Value.Get("createWritable").IsUndefined() {
		return fs.ErrNotSupported
	}
	h.writer, err = jsutil.AwaitErr(h.Value.Call("createWritable", map[string]any{"keepExistingData": h.append}))
	return
}

func (h *FileHandle) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return fs.ErrClosed
	}

	h.closed = true

	if !h.writer.IsUndefined() {
		_, err := jsutil.AwaitErr(h.writer.Call("close"))
		if err != nil {
			return err
		}

		// Invalidate stat cache since closing a writer commits changes
		h.fsys.invalidateCachedStat(h.path)
	}

	return nil
}

func (h *FileHandle) Name() string {
	return h.Value.Get("name").String()
}

// not applied until closed (like other writes)
func (h *FileHandle) Truncate(size int64) error {
	if err := h.tryCreateWritable(); err != nil {
		return err
	}
	if !h.writer.IsUndefined() {
		jsutil.Await(h.writer.Call("write", map[string]any{
			"type": "truncate",
			"size": size,
		}))

		// Invalidate stat cache since file size changed
		h.fsys.invalidateCachedStat(h.path)

		return nil
	}
	return fs.ErrPermission
}

func (h *FileHandle) Size() int64 {
	h.tryGetFile()
	return int64(h.file.Get("size").Int())
}

func (h *FileHandle) Stat() (fs.FileInfo, error) {
	// Check cache first (similar to httpfs pattern)
	if info, err, found := h.fsys.getCachedStat(h.path); found {
		if err != nil {
			return nil, err
		}
		return info, nil
	}

	// Build fresh stat from JS API + metadata store
	info, err := h.fsys.buildFileInfo(h.path, h.Value)
	if err != nil {
		// Cache the error
		h.fsys.setCachedStatError(h.path, err)
		return nil, err
	}

	// Cache the result
	h.fsys.setCachedStat(h.path, info)
	return info, nil
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

	if h.writer.IsUndefined() {
		return 0, &fs.PathError{Op: "write", Path: h.path, Err: fs.ErrPermission}
	}

	jsbuf := js.Global().Get("Uint8Array").New(len(b))
	n := js.CopyBytesToJS(jsbuf, b)

	// log.Println("fsa: write:", h.path, h.offset, n)
	_, err := jsutil.AwaitErr(h.writer.Call("write", map[string]any{
		"type":     "write",
		"data":     jsbuf,
		"position": h.offset,
	}))
	if err != nil {
		return 0, err
	}
	h.offset += int64(n)

	// Invalidate stat cache since file size/mtime changed
	h.fsys.invalidateCachedStat(h.path)

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
		return 0, &fs.PathError{Op: "read", Path: h.path, Err: fs.ErrInvalid}
	}
	rest := int(size - h.offset)
	if len(b) < rest {
		rest = len(b)
	}
	restblob := h.file.Call("slice", h.offset)
	arrbuf := jsutil.Await(restblob.Call("arrayBuffer"))
	jsbuf := js.Global().Get("Uint8Array").New(arrbuf)
	n := js.CopyBytesToGo(b, jsbuf)
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
		return 0, &fs.PathError{Op: "seek", Path: h.path, Err: fs.ErrInvalid}
	}
	h.offset = offset
	return offset, nil
}
