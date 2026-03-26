package term

import (
	"context"
	"io"
	iofs "io/fs"
	"os"
	"sync"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

const noExclude int64 = -1

type winchHub struct {
	mu      sync.Mutex
	readers map[int64]chan []byte
	nextID  int64
}

func newWinchHub() *winchHub {
	return &winchHub{readers: make(map[int64]chan []byte)}
}

func (h *winchHub) addReader() (id int64, ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	id = h.nextID
	ch = make(chan []byte, 64)
	h.readers[id] = ch
	return id, ch
}

func (h *winchHub) removeReader(id int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ch, ok := h.readers[id]; ok {
		delete(h.readers, id)
		close(ch)
	}
}

func (h *winchHub) broadcast(data []byte, exclude int64) {
	h.mu.Lock()
	var targets []chan []byte
	for id, ch := range h.readers {
		if exclude >= 0 && id == exclude {
			continue
		}
		targets = append(targets, ch)
	}
	h.mu.Unlock()
	payload := append([]byte(nil), data...)
	for _, ch := range targets {
		ch <- payload
	}
}

func (h *winchHub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.readers {
		close(ch)
	}
	h.readers = make(map[int64]chan []byte)
}

type winchFS struct {
	hub *winchHub
}

var (
	_ fs.FS            = (*winchFS)(nil)
	_ fs.ResolveFS     = (*winchFS)(nil)
	_ fs.OpenFileFS    = (*winchFS)(nil)
	_ fs.OpenContextFS = (*winchFS)(nil)
)

func (w *winchFS) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	if name == "." {
		return w, ".", nil
	}
	return nil, "", &fs.PathError{Op: "resolve", Path: name, Err: fs.ErrNotExist}
}

func (w *winchFS) Open(name string) (iofs.File, error) {
	return w.OpenContext(context.Background(), name)
}

func (w *winchFS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	return w.OpenFile(name, os.O_RDONLY, 0)
}

func (w *winchFS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	if name != "." {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	switch flag & 0x3 {
	case os.O_WRONLY:
		return &winchWriter{hub: w.hub}, nil
	case os.O_RDWR:
		return &winchRDWR{hub: w.hub}, nil
	default:
		return &winchReader{hub: w.hub}, nil
	}
}

type winchReader struct {
	hub    *winchHub
	once   sync.Once
	id     int64
	ch     chan []byte
	buf    []byte
	closed bool
	mu     sync.Mutex
}

func (r *winchReader) subscribe() {
	r.once.Do(func() {
		r.id, r.ch = r.hub.addReader()
	})
}

func (r *winchReader) Read(p []byte) (int, error) {
	r.subscribe()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return 0, fs.ErrClosed
	}
	if len(r.buf) > 0 {
		n := copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}
	frame, ok := <-r.ch
	if !ok {
		return 0, io.EOF
	}
	n := copy(p, frame)
	if n < len(frame) {
		r.buf = append([]byte(nil), frame[n:]...)
	}
	return n, nil
}

func (r *winchReader) Write(b []byte) (int, error) {
	return 0, fs.ErrPermission
}

func (r *winchReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	if r.ch != nil {
		r.hub.removeReader(r.id)
	}
	return nil
}

func (r *winchReader) Stat() (fs.FileInfo, error) {
	return fskit.Entry("winch", 0644), nil
}

type winchWriter struct {
	hub *winchHub
}

func (w *winchWriter) Read(p []byte) (int, error) {
	return 0, fs.ErrPermission
}

func (w *winchWriter) Write(b []byte) (int, error) {
	w.hub.broadcast(b, noExclude)
	return len(b), nil
}

func (w *winchWriter) Close() error { return nil }

func (w *winchWriter) Stat() (fs.FileInfo, error) {
	return fskit.Entry("winch", 0644), nil
}

type winchRDWR struct {
	hub    *winchHub
	mu     sync.Mutex
	id     int64
	ch     chan []byte
	buf    []byte
	closed bool
}

func (f *winchRDWR) ensure() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.id == 0 && !f.closed {
		f.id, f.ch = f.hub.addReader()
	}
}

func (f *winchRDWR) Read(p []byte) (int, error) {
	f.ensure()
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return 0, fs.ErrClosed
	}
	if len(f.buf) > 0 {
		n := copy(p, f.buf)
		f.buf = f.buf[n:]
		f.mu.Unlock()
		return n, nil
	}
	f.mu.Unlock()
	frame, ok := <-f.ch
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return 0, fs.ErrClosed
	}
	if !ok {
		f.mu.Unlock()
		return 0, io.EOF
	}
	n := copy(p, frame)
	if n < len(frame) {
		f.buf = append([]byte(nil), frame[n:]...)
	}
	f.mu.Unlock()
	return n, nil
}

func (f *winchRDWR) Write(b []byte) (int, error) {
	f.ensure()
	f.hub.broadcast(b, f.id)
	return len(b), nil
}

func (f *winchRDWR) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	if f.id != 0 {
		id := f.id
		f.mu.Unlock()
		f.hub.removeReader(id)
		f.mu.Lock()
	}
	return nil
}

func (f *winchRDWR) Stat() (fs.FileInfo, error) {
	return fskit.Entry("winch", 0644), nil
}
