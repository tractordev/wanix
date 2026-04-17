//go:build !js || !wasm

package tcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/internal"
)

type connState string

const (
	stateIdle      connState = "idle"
	stateBound     connState = "bound"
	stateListening connState = "listening"
	stateConnected connState = "connected"
	stateClosed    connState = "closed"
)

type Conn struct {
	id  string
	svc *Service

	mu sync.RWMutex

	state   connState
	network string // "tcp" or "unix"

	bindAddr string

	ln   net.Listener
	conn net.Conn

	lastErr error
}

func newConn(id string, svc *Service) *Conn {
	return &Conn{
		id:    id,
		svc:   svc,
		state: stateIdle,
	}
}

func (c *Conn) shutdown() {
	c.mu.Lock()
	ln := c.ln
	conn := c.conn
	c.ln = nil
	c.conn = nil
	c.state = stateClosed
	c.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
	if ln != nil {
		_ = ln.Close()
	}
}

func (c *Conn) Open(name string) (fs.File, error) {
	return c.OpenContext(context.Background(), name)
}

func (c *Conn) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, rname, err := c.ResolveFS(ctx, name)
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, rname)
}

func (c *Conn) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	fsys := fskit.MapFS{
		"ctl": internal.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the connection",
			Run: func(ctx *cli.Context, args []string) {
				if len(args) == 0 {
					return
				}
				switch args[0] {
				case "dial":
					if len(args) != 2 {
						panic(fmt.Errorf("usage: dial <addr>"))
					}
					if err := c.dial(args[1]); err != nil {
						panic(err)
					}
				case "bind":
					if len(args) != 2 {
						panic(fmt.Errorf("usage: bind <addr>"))
					}
					if err := c.bind(args[1]); err != nil {
						panic(err)
					}
				case "announce":
					if len(args) != 2 {
						panic(fmt.Errorf("usage: announce <addr>"))
					}
					if err := c.announce(args[1]); err != nil {
						panic(err)
					}
				case "hangup":
					if err := c.hangup(); err != nil {
						panic(err)
					}
				default:
					panic(fs.ErrNotSupported)
				}
			},
		}),
		"status": internal.FieldFile(func() (string, error) { return c.status(), nil }),
		"local":  internal.FieldFile(func() (string, error) { return c.local(), nil }),
		"remote": internal.FieldFile(func() (string, error) { return c.remote(), nil }),
		"data": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			if name != "." {
				return nil, fs.ErrNotExist
			}
			c.mu.RLock()
			conn := c.conn
			state := c.state
			c.mu.RUnlock()
			if state != stateConnected || conn == nil {
				return nil, fs.ErrPermission
			}
			return fskit.NewStreamFile(conn, conn, nil, "data", fs.FileMode(0644)), nil
		}),
	}

	// dynamically expose listen file only in listening mode
	c.mu.RLock()
	listening := c.state == stateListening && c.ln != nil
	c.mu.RUnlock()
	if listening {
		fsys["listen"] = fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			if name != "." {
				return nil, fs.ErrNotExist
			}
			return newListenFile(c, ctx), nil
		})
	}

	return fs.Resolve(fsys, ctx, name)
}

func isUnixPathAddr(addr string) bool {
	if addr == "" {
		return false
	}
	if strings.HasPrefix(addr, "/") || strings.HasPrefix(addr, ".") {
		return true
	}
	// windows-ish or explicit relative path segments
	if strings.Contains(addr, string(filepath.Separator)) {
		return true
	}
	return false
}

func (c *Conn) bind(addr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == stateConnected || c.state == stateListening {
		return fmt.Errorf("bind: %w", fs.ErrPermission)
	}
	c.bindAddr = addr
	c.network = networkForAddr(addr)
	c.state = stateBound
	return nil
}

func networkForAddr(addr string) string {
	if isUnixPathAddr(addr) {
		return "unix"
	}
	return "tcp"
}

func (c *Conn) dial(addr string) error {
	network := networkForAddr(addr)

	c.mu.Lock()
	if c.state == stateConnected || c.state == stateListening {
		c.mu.Unlock()
		return fmt.Errorf("dial: %w", fs.ErrPermission)
	}
	bindAddr := c.bindAddr
	c.mu.Unlock()

	var d net.Dialer
	if bindAddr != "" {
		if network == "tcp" && !isUnixPathAddr(bindAddr) {
			la, err := net.ResolveTCPAddr("tcp", bindAddr)
			if err != nil {
				return err
			}
			d.LocalAddr = la
		} else if network == "unix" && isUnixPathAddr(bindAddr) {
			d.LocalAddr = &net.UnixAddr{Name: bindAddr, Net: "unix"}
		}
	}

	conn, err := d.Dial(network, addr)
	if err != nil {
		c.mu.Lock()
		c.lastErr = err
		c.mu.Unlock()
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.ln = nil
	c.network = network
	c.state = stateConnected
	c.lastErr = nil
	c.mu.Unlock()
	return nil
}

func (c *Conn) announce(addr string) error {
	network := networkForAddr(addr)

	c.mu.Lock()
	if c.state == stateConnected || c.state == stateListening {
		c.mu.Unlock()
		return fmt.Errorf("announce: %w", fs.ErrPermission)
	}
	c.mu.Unlock()

	if network == "unix" {
		// best-effort cleanup; if the path doesn't exist that's fine
		_ = os.Remove(addr)
	}
	ln, err := net.Listen(network, addr)
	if err != nil {
		c.mu.Lock()
		c.lastErr = err
		c.mu.Unlock()
		return err
	}

	c.mu.Lock()
	c.ln = ln
	c.conn = nil
	c.network = network
	c.state = stateListening
	c.lastErr = nil
	c.mu.Unlock()
	return nil
}

func (c *Conn) hangup() error {
	c.mu.Lock()
	ln := c.ln
	conn := c.conn
	c.ln = nil
	c.conn = nil
	c.bindAddr = ""
	c.lastErr = nil
	c.state = stateClosed
	c.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	if ln != nil {
		_ = ln.Close()
	}
	return nil
}

func (c *Conn) status() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var b strings.Builder
	b.WriteString(string(c.state))
	if c.conn != nil {
		b.WriteString(" local=")
		b.WriteString(addrString(c.conn.LocalAddr()))
		b.WriteString(" remote=")
		b.WriteString(addrString(c.conn.RemoteAddr()))
	} else if c.ln != nil {
		b.WriteString(" local=")
		b.WriteString(addrString(c.ln.Addr()))
	}
	if c.lastErr != nil {
		b.WriteString(" err=")
		b.WriteString(c.lastErr.Error())
	}
	return b.String()
}

func (c *Conn) local() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn != nil {
		return addrString(c.conn.LocalAddr())
	}
	if c.ln != nil {
		return addrString(c.ln.Addr())
	}
	if c.bindAddr != "" {
		return c.bindAddr
	}
	return ""
}

func (c *Conn) remote() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn != nil {
		return addrString(c.conn.RemoteAddr())
	}
	return ""
}

func addrString(a net.Addr) string {
	if a == nil {
		return ""
	}
	// net.Addr.String() is already the canonical form for tcp/unix
	return a.String()
}

type listenFile struct {
	c   *Conn
	ctx context.Context

	once sync.Once
	buf  *bytes.Reader
	err  error
	node *fskit.Node
}

func newListenFile(c *Conn, ctx context.Context) *listenFile {
	return &listenFile{
		c:   c,
		ctx: ctx,
		node: fskit.Entry("listen", 0444,
			-1,
		),
	}
}

func (f *listenFile) Stat() (fs.FileInfo, error) { return f.node, nil }
func (f *listenFile) Close() error               { return nil }

func (f *listenFile) Read(p []byte) (int, error) {
	f.once.Do(func() {
		rid, err := f.c.acceptOne(f.ctx)
		f.err = err
		if err != nil {
			return
		}
		f.buf = bytes.NewReader([]byte(rid + "\n"))
	})
	if f.err != nil {
		return 0, f.err
	}
	if f.buf == nil {
		return 0, io.EOF
	}
	return f.buf.Read(p)
}

func (c *Conn) acceptOne(ctx context.Context) (string, error) {
	c.mu.RLock()
	ln := c.ln
	if c.state != stateListening || ln == nil {
		c.mu.RUnlock()
		return "", fs.ErrPermission
	}
	c.mu.RUnlock()

	// net.Listener.Accept is blocking; context cancellation is best-effort:
	// if ctx is canceled, we close the listener to unblock.
	type res struct {
		conn net.Conn
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		conn, err := ln.Accept()
		ch <- res{conn: conn, err: err}
	}()

	select {
	case <-ctx.Done():
		_ = ln.Close()
		r := <-ch
		if r.conn != nil {
			_ = r.conn.Close()
		}
		if r.err != nil && !errors.Is(r.err, net.ErrClosed) {
			return "", r.err
		}
		return "", ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return "", r.err
		}
		rid, err := c.svc.Alloc()
		if err != nil {
			_ = r.conn.Close()
			return "", err
		}
		cc, err := c.svc.Get(rid)
		if err != nil {
			_ = r.conn.Close()
			return "", err
		}
		cc.mu.Lock()
		cc.conn = r.conn
		cc.ln = nil
		cc.bindAddr = ""
		cc.network = c.network
		cc.state = stateConnected
		cc.lastErr = nil
		cc.mu.Unlock()
		return rid, nil
	}
}

// Ensure listenFile implements fs.File
var _ fs.File = (*listenFile)(nil)
var _ = (io.Reader)((*listenFile)(nil))

