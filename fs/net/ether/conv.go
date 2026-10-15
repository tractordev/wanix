package ether

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/misc"
)

const (
	ethHeaderLen = 14
	// frames queued for reading on data before new ones are dropped
	rxQueueLen = 256
)

// Conv is a conversation: a virtual NIC attached to the vnet switch
// via an AcceptQemu session. It lives as long as it has open files.
type Conv struct {
	id  string
	svc *Service

	mu    sync.Mutex
	refs  int
	freed bool
	err   error

	ctx    context.Context
	cancel context.CancelFunc
	conn   net.Conn // our end of the pipe; the other end is given to AcceptQemu
	wmu    sync.Mutex
	frames chan []byte
	done   chan struct{}
	once   sync.Once

	in, out, inBytes, outBytes, dropped atomic.Uint64
}

func newConv(id string, svc *Service) *Conv {
	ctx, cancel := context.WithCancel(context.Background())
	return &Conv{
		id:     id,
		svc:    svc,
		refs:   1,
		ctx:    ctx,
		cancel: cancel,
		frames: make(chan []byte, rxQueueLen),
		done:   make(chan struct{}),
	}
}

func (c *Conv) start() {
	local, remote := net.Pipe()
	c.conn = local
	go func() {
		c.end(c.svc.vn.AcceptQemu(c.ctx, remote))
	}()
	go c.rxLoop()
}

// rxLoop drains frames from the switch so it never blocks on a slow reader.
func (c *Conv) rxLoop() {
	var size [4]byte
	for {
		if _, err := io.ReadFull(c.conn, size[:]); err != nil {
			c.end(err)
			return
		}
		frame := make([]byte, binary.BigEndian.Uint32(size[:]))
		if _, err := io.ReadFull(c.conn, frame); err != nil {
			c.end(err)
			return
		}
		select {
		case c.frames <- frame:
		default:
			c.dropped.Add(1)
		}
	}
}

// end terminates the session. The conversation stays until its last handle closes.
func (c *Conv) end(err error) {
	c.once.Do(func() {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, context.Canceled) {
			err = nil
		}
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		c.cancel()
		close(c.done)
		if c.conn != nil {
			_ = c.conn.Close()
		}
	})
}

func (c *Conv) isDone() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *Conv) acquire() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.freed {
		return fs.ErrNotExist
	}
	c.refs++
	return nil
}

func (c *Conv) release() {
	c.mu.Lock()
	c.refs--
	free := c.refs <= 0 && !c.freed
	if free {
		c.freed = true
	}
	c.mu.Unlock()
	if free {
		c.end(nil)
		c.svc.remove(c.id)
	}
}

func (c *Conv) Open(name string) (fs.File, error) {
	return c.OpenContext(context.Background(), name)
}

func (c *Conv) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if name == "." {
		return fs.OpenContext(ctx, c.rootFS(), name)
	}
	if err := c.acquire(); err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	f, err := fs.OpenContext(ctx, c.rootFS(), name)
	if err != nil {
		c.release()
		return nil, err
	}
	return c.wrap(f), nil
}

func (c *Conv) Stat(name string) (fs.FileInfo, error) {
	return c.StatContext(context.Background(), name)
}

func (c *Conv) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	return fs.StatContext(ctx, c.rootFS(), name)
}

func (c *Conv) rootFS() fskit.MapFS {
	return fskit.MapFS{
		"ctl": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			if name != "." {
				return nil, fs.ErrNotExist
			}
			return newCtlFile(c), nil
		}),
		"data": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			if name != "." {
				return nil, fs.ErrNotExist
			}
			return &dataFile{c: c}, nil
		}),
		"type":  misc.FieldFile("-1"),
		"stats": misc.FieldFile(func() (string, error) { return c.stats(), nil }),
	}
}

func (c *Conv) stats() string {
	c.mu.Lock()
	err := c.err
	c.mu.Unlock()
	state := "open"
	if c.isDone() {
		state = "closed"
		if err != nil {
			state = fmt.Sprintf("closed (%s)", err)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "in: %d\n", c.in.Load())
	fmt.Fprintf(&b, "out: %d\n", c.out.Load())
	fmt.Fprintf(&b, "inbytes: %d\n", c.inBytes.Load())
	fmt.Fprintf(&b, "outbytes: %d\n", c.outBytes.Load())
	fmt.Fprintf(&b, "dropped: %d\n", c.dropped.Load())
	fmt.Fprintf(&b, "state: %s\n", state)
	return b.String()
}

func (c *Conv) ctl(line string) error {
	args := strings.Fields(line)
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "connect":
		if len(args) != 2 {
			return fmt.Errorf("usage: connect <type>")
		}
		if args[1] != "-1" {
			return fmt.Errorf("connect %s: %w", args[1], fs.ErrNotSupported)
		}
		return nil
	case "hangup":
		c.end(nil)
		return nil
	default:
		return fmt.Errorf("%s: %w", args[0], fs.ErrNotSupported)
	}
}

func (c *Conv) send(frame []byte) (int, error) {
	if len(frame) < ethHeaderLen || len(frame) > c.svc.maxFrame() {
		return 0, fmt.Errorf("frame size %d: %w", len(frame), fs.ErrInvalid)
	}
	if c.isDone() {
		return 0, io.ErrClosedPipe
	}
	buf := make([]byte, 4+len(frame))
	binary.BigEndian.PutUint32(buf, uint32(len(frame)))
	copy(buf[4:], frame)
	c.wmu.Lock()
	_, err := c.conn.Write(buf)
	c.wmu.Unlock()
	if err != nil {
		return 0, io.ErrClosedPipe
	}
	c.out.Add(1)
	c.outBytes.Add(uint64(len(frame)))
	return len(frame), nil
}

func (c *Conv) recv(p []byte) (int, error) {
	if c.isDone() {
		return 0, io.EOF
	}
	select {
	case <-c.done:
		return 0, io.EOF
	case frame := <-c.frames:
		// frames larger than p are truncated, as in Plan 9
		n := copy(p, frame)
		c.in.Add(1)
		c.inBytes.Add(uint64(n))
		return n, nil
	}
}

// wrap returns f with Close releasing a reference on c.
func (c *Conv) wrap(f fs.File) fs.File {
	return &convFile{File: f, c: c}
}

type convFile struct {
	fs.File
	c    *Conv
	once sync.Once
}

func (f *convFile) Close() error {
	err := f.File.Close()
	f.once.Do(f.c.release)
	return err
}

func (f *convFile) Identity() fs.ID                           { return fs.Identity(f.File) }
func (f *convFile) Write(b []byte) (int, error)               { return fs.Write(f.File, b) }
func (f *convFile) ReadAt(b []byte, off int64) (int, error)   { return fs.ReadAt(f.File, b, off) }
func (f *convFile) WriteAt(b []byte, off int64) (int, error)  { return fs.WriteAt(f.File, b, off) }
func (f *convFile) Seek(off int64, whence int) (int64, error) { return fs.Seek(f.File, off, whence) }

// ctlFile executes each written line as a command. Reading returns the id.
type ctlFile struct {
	c   *Conv
	mu  sync.Mutex
	off int64
}

func newCtlFile(c *Conv) *ctlFile {
	return &ctlFile{c: c}
}

func (f *ctlFile) Stat() (fs.FileInfo, error) { return fskit.Entry("ctl", 0644), nil }
func (f *ctlFile) Close() error               { return nil }

func (f *ctlFile) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, err := f.ReadAt(p, f.off)
	f.off += int64(n)
	return n, err
}

func (f *ctlFile) ReadAt(p []byte, off int64) (int, error) {
	data := f.c.id + "\n"
	if off >= int64(len(data)) {
		return 0, io.EOF
	}
	return copy(p, data[off:]), nil
}

func (f *ctlFile) Seek(off int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch whence {
	case io.SeekStart:
		f.off = off
	case io.SeekCurrent:
		f.off += off
	default:
		return 0, fs.ErrNotSupported
	}
	return f.off, nil
}

func (f *ctlFile) Write(p []byte) (int, error) {
	for _, line := range strings.Split(string(p), "\n") {
		if err := f.c.ctl(line); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (f *ctlFile) WriteAt(p []byte, off int64) (int, error) { return f.Write(p) }

// dataFile reads and writes one Ethernet frame per call.
type dataFile struct {
	c *Conv
}

func (f *dataFile) Stat() (fs.FileInfo, error)               { return fskit.Entry("data", 0644), nil }
func (f *dataFile) Close() error                             { return nil }
func (f *dataFile) Read(p []byte) (int, error)               { return f.c.recv(p) }
func (f *dataFile) ReadAt(p []byte, off int64) (int, error)  { return f.c.recv(p) }
func (f *dataFile) Write(p []byte) (int, error)              { return f.c.send(p) }
func (f *dataFile) WriteAt(p []byte, off int64) (int, error) { return f.c.send(p) }
