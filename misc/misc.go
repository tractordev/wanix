package misc

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"sync/atomic"
	"unicode"

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

// EncodeVector encodes v as NUL-terminated entries; an empty vector is empty data.
func EncodeVector(v []string) ([]byte, error) {
	if len(v) == 0 {
		return nil, nil
	}
	for _, s := range v {
		if strings.Contains(s, "\x00") {
			return nil, errors.New("embedded NUL in vector entry")
		}
	}
	return []byte(strings.Join(v, "\x00") + "\x00"), nil
}

// DecodeVector decodes NUL-terminated entries; empty data is an empty vector.
func DecodeVector(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(b), "\x00"), "\x00")
}

// JoinArgs shell-quotes each entry and joins them with one space.
func JoinArgs(v []string) string {
	quoted := make([]string, len(v))
	for i, s := range v {
		switch {
		case s == "":
			quoted[i] = "''"
		case strings.ContainsAny(s, "'\"\\") || strings.IndexFunc(s, unicode.IsSpace) >= 0:
			quoted[i] = "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
		default:
			quoted[i] = s
		}
	}
	return strings.Join(quoted, " ")
}

// VectorFile is FieldFile for NUL vectors: it adds no trailing newline.
// Each open seeds the node from the literal value (nil unless one is given),
// so the setter replaces the whole vector: a second open+write drops the first.
func VectorFile(args ...any) fs.FS {
	var (
		mode   fs.FileMode = 0555
		value  []string
		getter func() []string
		setter func([]string) error
	)
	for _, arg := range args {
		switch v := arg.(type) {
		case []string:
			value = v
		case func() []string:
			getter = v
		case func([]string) error:
			setter = v
		case fs.FileMode:
			mode = v
		}
	}
	if setter != nil && mode == 0555 {
		mode = 0755
	}
	return fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
		var wasRead atomic.Bool
		data, _ := EncodeVector(value)
		return &fskit.FuncFile{
			Node: fskit.Entry(name, mode, -1, data),
			ReadFunc: func(n *fskit.Node) error {
				wasRead.Store(true)
				if getter != nil {
					// shortcut: the read path drops EncodeVector's error, so a
					// getter entry with an embedded NUL reads as an empty file,
					// upgrade if a getter can hold raw bytes.
					b, _ := EncodeVector(getter())
					fskit.SetData(n, b)
				}
				return nil
			},
			CloseFunc: func(n *fskit.Node) error {
				if setter != nil && !wasRead.Load() {
					data := n.Data()
					if len(data) > 0 && data[len(data)-1] != '\x00' {
						return fmt.Errorf("%s: vector data must end with NUL", name)
					}
					return setter(DecodeVector(data))
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
