package signal

import (
	"context"
	"io"
	iofs "io/fs"
	"os"
	"sync"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/vfs"
)

// FS exposes a single virtual file at ".". O_RDONLY opens a read-only
// subscriber; O_WRONLY opens a writer that broadcasts to all subscribers;
// O_RDWR opens a combined end that does not receive its own writes.
type FS struct {
	b *Broadcaster
}

var (
	_ fs.FS            = (*FS)(nil)
	_ fs.ResolveFS     = (*FS)(nil)
	_ fs.OpenFileFS    = (*FS)(nil)
	_ fs.OpenContextFS = (*FS)(nil)
)

// NewFS wraps an existing broadcaster. b must be non-nil.
func NewFS(b *Broadcaster) *FS {
	return &FS{b: b}
}

// New returns a new filesystem backed by its own broadcaster, for callers
// that need both the fs view and direct Broadcast/Close access.
func New() (fs.FS, *Broadcaster) {
	b := NewBroadcaster()
	return NewFS(b), b
}

func (w *FS) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	if name == "." {
		return w, ".", nil
	}
	return nil, "", &fs.PathError{Op: "resolve", Path: name, Err: fs.ErrNotExist}
}

func (w *FS) Open(name string) (iofs.File, error) {
	return w.OpenContext(context.Background(), name)
}

func (w *FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	return w.OpenFile(name, os.O_RDONLY, 0)
}

func (w *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	if name != "." {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	switch flag & 0x3 {
	case os.O_WRONLY:
		return &writer{b: w.b}, nil
	case os.O_RDWR:
		return &rdwr{b: w.b}, nil
	default:
		return &reader{b: w.b}, nil
	}
}

type reader struct {
	b      *Broadcaster
	once   sync.Once
	id     int64
	ch     chan []byte
	buf    []byte
	closed bool
	mu     sync.Mutex
}

func (r *reader) subscribe() {
	r.once.Do(func() {
		r.id, r.ch = r.b.AddReader()
	})
}

func (r *reader) Read(p []byte) (int, error) {
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

func (r *reader) Write(b []byte) (int, error) {
	return 0, fs.ErrPermission
}

func (r *reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	if r.ch != nil {
		r.b.RemoveReader(r.id)
	}
	return nil
}

func (r *reader) Stat() (fs.FileInfo, error) {
	return fskit.Entry("signal", 0644), nil
}

type writer struct {
	b *Broadcaster
}

func (w *writer) Read(p []byte) (int, error) {
	return 0, fs.ErrPermission
}

func (w *writer) Write(b []byte) (int, error) {
	w.b.Broadcast(b, NoExclude)
	return len(b), nil
}

func (w *writer) Close() error { return nil }

func (w *writer) Stat() (fs.FileInfo, error) {
	return fskit.Entry("signal", 0644), nil
}

type rdwr struct {
	b      *Broadcaster
	mu     sync.Mutex
	id     int64
	ch     chan []byte
	buf    []byte
	closed bool
}

func (f *rdwr) ensure() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.id == 0 && !f.closed {
		f.id, f.ch = f.b.AddReader()
	}
}

func (f *rdwr) Read(p []byte) (int, error) {
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

func (f *rdwr) Write(b []byte) (int, error) {
	f.ensure()
	f.b.Broadcast(b, f.id)
	return len(b), nil
}

func (f *rdwr) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	if f.id != 0 {
		id := f.id
		f.mu.Unlock()
		f.b.RemoveReader(id)
		f.mu.Lock()
	}
	return nil
}

func (f *rdwr) Stat() (fs.FileInfo, error) {
	return fskit.Entry("signal", 0644), nil
}

// Allocator yields a fresh signal FS (and broadcaster) per bind, like #pipe.
type Allocator struct{}

func (a *Allocator) Open(name string) (iofs.File, error) {
	return a.OpenContext(context.Background(), name)
}

func (a *Allocator) OpenContext(ctx context.Context, name string) (fs.File, error) {
	return fskit.RawNode(name, 0644).OpenContext(ctx, name)
}

func (a *Allocator) BindAllocFS(name string) (fs.FS, error) {
	b := NewBroadcaster()
	return NewFS(b), nil
}

var _ vfs.BindAllocator = (*Allocator)(nil)
