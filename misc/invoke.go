package misc

import (
	"bytes"
	"io"
	"io/fs"
	"sync"
	"time"

	"tractor.dev/wanix/fs/fskit"
)

// this read timeout makes sure read doesnt block forever and provides backwards compat with old read only protocol for new/alloc files
const invokeReadTimeout = 2 * time.Second

// InvokeFile is a file-like struct that blocks Read until Write is called at least once.
// The first Write argument string (until newline) is sent to the Read and unblocks it.
// If only a newline or a zero-length write occurs, Read unblocks and returns "Hello world\n".
// Read returns ErrReadTimeout if Write does not arrive within 5 seconds.
type InvokeFile struct {
	mu       sync.Mutex
	cond     *sync.Cond
	buf      []byte
	ready    bool
	closed   bool
	timedOut bool
	fn       InvokeFunc
}

// InvokeFunc is a function that takes a map of options and returns an error.
// Eventually we can make it more generic...
type InvokeFunc func(opts map[string]string) (string, error)

func NewInvokeFile(fn InvokeFunc) *InvokeFile {
	f := &InvokeFile{
		fn: fn,
	}
	f.cond = sync.NewCond(&f.mu)
	return f
}

func (f *InvokeFile) Stat() (fs.FileInfo, error) {
	return fskit.Entry("fn", 0644), nil
}

// Write takes a string argument (ending with a newline). This unblocks Read.
func (f *InvokeFile) Write(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ready || f.closed {
		return 0, io.ErrClosedPipe
	}
	msg := p
	if idx := bytes.IndexByte(msg, '\n'); idx >= 0 {
		msg = msg[:idx]
	}

	opts := make(map[string]string)
	fields := bytes.Fields(msg)
	for _, field := range fields {
		kv := bytes.SplitN(field, []byte("="), 2)
		if len(kv) == 2 {
			opts[string(kv[0])] = string(kv[1])
		} else if len(kv) == 1 {
			opts[string(kv[0])] = ""
		}
	}

	res, err := f.fn(opts)
	if err != nil {
		return 0, err
	}
	f.buf = []byte(res + "\n")

	f.ready = true
	f.cond.Broadcast()
	return len(p), nil
}

// Read blocks until Write is called at least once. Then delivers the written string as bytes.
func (f *InvokeFile) Read(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	timedOut := false

	if !f.ready && !f.closed {
		done := make(chan struct{})
		timer := time.AfterFunc(invokeReadTimeout, func() {
			f.mu.Lock()
			defer f.mu.Unlock()
			if !f.ready && !f.closed {
				f.timedOut = true
				timedOut = true // local variable
				f.cond.Broadcast()
			}
		})
		defer timer.Stop()

		for !f.ready && !f.closed && !timedOut {
			f.cond.Wait()
		}
		close(done)
	}

	// If we timed out, run f.fn() once, but do not re-trigger the timedout loop
	if f.timedOut {
		// Only run this once per timeout event
		f.timedOut = false // so we don't loop on next Read
		res, err := f.fn(make(map[string]string))
		if err != nil {
			return 0, err
		}
		f.closed = false
		f.buf = []byte(res + "\n")
	}

	if f.closed {
		return 0, io.EOF
	}
	if len(f.buf) == 0 {
		return 0, io.EOF
	}
	n = copy(p, f.buf)
	f.buf = f.buf[n:]
	if len(f.buf) == 0 {
		f.closed = true // file reads only once
	}
	return n, nil
}

func (f *InvokeFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	f.cond.Broadcast()
	return nil
}
