//go:build js && wasm

package jsutil

import (
	"bytes"
	"context"
	iofs "io/fs"
	"os"
	"sync"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

// ConsoleFS is a single virtual file at "." whose Write calls Log with the
// payload interpreted as a UTF-8 string. Read-only opens read as EOF.
var ConsoleFS fs.FS = &consoleFS{w: new(consoleWriter)}

type consoleFS struct {
	mu sync.Mutex
	w  *consoleWriter
	n  int // open handles; shared writer buffer persists across opens
}

var (
	_ fs.ResolveFS     = (*consoleFS)(nil)
	_ fs.OpenFileFS    = (*consoleFS)(nil)
	_ fs.OpenContextFS = (*consoleFS)(nil)
)

func (c *consoleFS) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	if name == "." {
		return c, ".", nil
	}
	return nil, "", &fs.PathError{Op: "resolve", Path: name, Err: fs.ErrNotExist}
}

func (c *consoleFS) Open(name string) (iofs.File, error) {
	return c.OpenContext(context.Background(), name)
}

func (c *consoleFS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	return c.OpenFile(name, os.O_RDONLY, 0)
}

func (c *consoleFS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	if name != "." {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return &consoleHandle{c: c}, nil
}

type consoleHandle struct {
	c *consoleFS
}

func (h *consoleHandle) Read(p []byte) (int, error) {
	return h.c.w.Read(p)
}

func (h *consoleHandle) Write(b []byte) (int, error) {
	return h.c.w.Write(b)
}

func (h *consoleHandle) Close() error {
	h.c.mu.Lock()
	h.c.n--
	last := h.c.n == 0
	h.c.mu.Unlock()
	if last {
		h.c.w.flushPartialLine()
	}
	return nil
}

func (h *consoleHandle) Stat() (fs.FileInfo, error) {
	return h.c.w.Stat()
}

type consoleWriter struct {
	mu  sync.Mutex
	buf []byte
}

func (w *consoleWriter) Read(p []byte) (int, error) {
	return 0, fs.ErrPermission
}

func (w *consoleWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, b...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := w.buf[:i]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		Log(string(line))
		w.buf = w.buf[i+1:]
	}
	return len(b), nil
}

func (w *consoleWriter) flushPartialLine() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) == 0 {
		return
	}
	line := w.buf
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	Log(string(line))
	w.buf = nil
}

func (w *consoleWriter) Stat() (fs.FileInfo, error) {
	return fskit.Entry("console", 0644), nil
}
