//go:build js && wasm

package jsfs

import (
	"bytes"
	"io"
	"path"
	"strings"
	"sync"
	"syscall"
	"syscall/js"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/misc/shlex"
)

type invokeOutcome struct {
	data []byte
	err  error
}

type funcFile struct {
	name string
	fn   js.Value
	this js.Value

	jsonMode bool

	mu      sync.Mutex
	lineBuf []byte
	closed  bool

	pending   []byte
	pendingAt int

	results chan invokeOutcome

	// After one invocation's response bytes are fully read, next Read returns (0, io.EOF)
	// so callers like io.Copy can stop without blocking for another invoke.
	readEOFAfterOutcome bool
}

func newFuncFile(name string, fn, this js.Value, jsonMode bool) *funcFile {
	return &funcFile{
		name:     name,
		fn:       fn,
		this:     this,
		jsonMode: jsonMode,
		results:  make(chan invokeOutcome, 256),
	}
}

func (f *funcFile) Stat() (fs.FileInfo, error) {
	return fskit.Entry(path.Base(f.name), 0755, 0), nil
}

func (f *funcFile) Read(b []byte) (int, error) {
	for {
		f.mu.Lock()
		if f.closed {
			f.mu.Unlock()
			return 0, fs.ErrClosed
		}
		if f.readEOFAfterOutcome {
			f.readEOFAfterOutcome = false
			f.mu.Unlock()
			return 0, io.EOF
		}
		if len(f.pending) > f.pendingAt {
			n := copy(b, f.pending[f.pendingAt:])
			f.pendingAt += n
			if f.pendingAt >= len(f.pending) {
				f.pending = nil
				f.pendingAt = 0
				f.readEOFAfterOutcome = true
			}
			f.mu.Unlock()
			return n, nil
		}
		f.mu.Unlock()

		r, ok := <-f.results
		if !ok {
			f.mu.Lock()
			closed := f.closed
			f.mu.Unlock()
			if closed {
				return 0, fs.ErrClosed
			}
			return 0, io.EOF
		}
		if r.err != nil {
			return 0, r.err
		}
		f.mu.Lock()
		f.pending = r.data
		f.pendingAt = 0
		if len(f.pending) == 0 {
			f.readEOFAfterOutcome = true
		}
		f.mu.Unlock()
	}
}

func (f *funcFile) Write(b []byte) (int, error) {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return 0, fs.ErrClosed
	}
	f.lineBuf = append(f.lineBuf, b...)
	var lines [][]byte
	for {
		idx := bytes.IndexByte(f.lineBuf, '\n')
		if idx < 0 {
			break
		}
		line := append([]byte(nil), f.lineBuf[:idx]...)
		f.lineBuf = f.lineBuf[idx+1:]
		lines = append(lines, line)
	}
	f.mu.Unlock()

	for _, line := range lines {
		out := f.invokeLine(line)
		f.results <- out
	}
	return len(b), nil
}

func (f *funcFile) invokeLine(line []byte) invokeOutcome {
	if f.jsonMode {
		s := strings.TrimSpace(string(line))
		arr, err := jsonParseArray(s)
		if err != nil {
			return invokeOutcome{err: syscall.EINVAL}
		}
		args, err := jsonArrayToArgs(arr)
		if err != nil {
			return invokeOutcome{err: syscall.EINVAL}
		}
		res, err := reflectApply(f.fn, f.this, args)
		if err != nil {
			return invokeOutcome{err: syscall.EIO}
		}
		if res.IsUndefined() {
			return invokeOutcome{}
		}
		data, err := jsonStringifyLine(res)
		if err != nil {
			return invokeOutcome{err: syscall.EIO}
		}
		return invokeOutcome{data: data}
	}

	s := string(line)
	toks, err := shlex.Split(s, true)
	if err != nil {
		return invokeOutcome{err: syscall.EINVAL}
	}
	args := make([]js.Value, 0, len(toks))
	for _, t := range toks {
		if strings.HasPrefix(t, "@") {
			rv, err := resolveGlobalPath(strings.TrimSpace(t))
			if err != nil {
				return invokeOutcome{err: syscall.EINVAL}
			}
			args = append(args, rv)
			continue
		}
		args = append(args, js.ValueOf(t))
	}
	res, err := reflectApply(f.fn, f.this, args)
	if err != nil {
		return invokeOutcome{err: syscall.EIO}
	}
	if res.IsUndefined() {
		return invokeOutcome{}
	}
	return invokeOutcome{data: jsToStringLine(res)}
}

func (f *funcFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	close(f.results)
	return nil
}

func (f *funcFile) Seek(int64, int) (int64, error) {
	return 0, &fs.PathError{Op: "seek", Path: f.name, Err: fs.ErrInvalid}
}
