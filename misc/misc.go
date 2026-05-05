package misc

import (
	"context"
	"io/fs"
	"strings"
	"sync/atomic"

	"errors"
	"io"
	"net"
	"time"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/misc/shlex"
)

// FakeConn adapts an io.ReadWriteCloser to a net.Conn (minimal implementation).
type FakeConn struct {
	rwc    io.ReadWriteCloser
	closed chan struct{}
}

// NewFakeConn wraps an io.ReadWriteCloser as a net.Conn.
func NewFakeConn(rwc io.ReadWriteCloser) net.Conn {
	return &FakeConn{
		rwc:    rwc,
		closed: make(chan struct{}),
	}
}

func (c *FakeConn) Read(b []byte) (int, error) {
	return c.rwc.Read(b)
}

func (c *FakeConn) Write(b []byte) (int, error) {
	return c.rwc.Write(b)
}

func (c *FakeConn) Close() error {
	select {
	case <-c.closed:
		return nil
	default:
		close(c.closed)
		return c.rwc.Close()
	}
}

func (c *FakeConn) LocalAddr() net.Addr {
	return dummyAddr("rwc-local")
}

func (c *FakeConn) RemoteAddr() net.Addr {
	return dummyAddr("rwc-remote")
}

func (c *FakeConn) SetDeadline(t time.Time) error {
	// Not supported
	return errors.New("SetDeadline not supported")
}

func (c *FakeConn) SetReadDeadline(t time.Time) error {
	// Not supported
	return errors.New("SetReadDeadline not supported")
}

func (c *FakeConn) SetWriteDeadline(t time.Time) error {
	// Not supported
	return errors.New("SetWriteDeadline not supported")
}

// dummyAddr implements net.Addr for placeholder addresses.
type dummyAddr string

func (a dummyAddr) Network() string { return "rwc" }
func (a dummyAddr) String() string  { return string(a) }

func FieldFile(args ...any) fs.FS {
	var (
		mode   fs.FileMode = 0555
		value  string
		getter func() (string, error)
		setter func([]byte) error
	)
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			value = v
		case func() (string, error):
			getter = v
		case func([]byte) error:
			setter = v
		case fs.FileMode:
			mode = v
		default:
			// no-op, skip
		}
	}
	if setter != nil && mode == 0555 {
		mode = 0755
	}
	return fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
		var wasRead atomic.Bool
		return &fskit.FuncFile{
			Node: fskit.Entry(name, mode, -1, []byte(value+"\n")),
			ReadFunc: func(n *fskit.Node) error {
				wasRead.Store(true)
				if getter != nil {
					v, err := getter()
					if err != nil {
						return err
					}
					if len(v) == 0 || v[len(v)-1] != '\n' {
						v = v + "\n"
					}
					fskit.SetData(n, []byte(v))
				}
				return nil
			},
			CloseFunc: func(n *fskit.Node) error {
				// only call setter if setter is set
				// and there was no read
				if setter != nil && !wasRead.Load() {
					return setter(n.Data())
				}
				return nil
			},
		}, nil
	})
}

func ControlFile(cmd *cli.Command) fs.FS {
	return fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
		return &fskit.FuncFile{
			Node: fskit.Entry(cmd.Name(), 0755),
			CloseFunc: func(n *fskit.Node) error {
				args, err := shlex.Split(strings.TrimSpace(string(n.Data())), true)
				if err != nil {
					return err
				}
				if len(args) == 0 {
					return nil
				}
				return cli.Execute(ctx, cmd, args)
			},
		}, nil
	})
}
