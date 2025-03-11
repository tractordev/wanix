package internal

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"net"
	"strings"
	"sync"
	"time"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs/fskit"
)

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
		var wasRead bool
		return &fskit.FuncFile{
			Node: fskit.Entry(name, mode, -1, []byte(value+"\n")),
			ReadFunc: func(n *fskit.Node) error {
				wasRead = true
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
				if setter != nil && !wasRead {
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
				args := strings.Split(strings.TrimSpace(string(n.Data())), " ")
				if len(args) == 0 {
					return nil
				}
				return cli.Execute(ctx, cmd, args)
			},
		}, nil
	})
}

type BufferedPipe struct {
	buffer   bytes.Buffer
	mu       sync.Mutex
	dataCond *sync.Cond
	closed   bool
	block    bool
}

func NewBufferedPipe(block bool) *BufferedPipe {
	bp := &BufferedPipe{block: block}
	bp.dataCond = sync.NewCond(&bp.mu)
	return bp
}

func (bp *BufferedPipe) Write(data []byte) (int, error) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.closed {
		return 0, io.ErrClosedPipe
	}

	n, err := bp.buffer.Write(data)
	bp.dataCond.Signal()
	return n, err
}

func (bp *BufferedPipe) Read(p []byte) (int, error) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.block {
		for bp.buffer.Len() == 0 && !bp.closed {
			bp.dataCond.Wait()
		}
	}

	if bp.closed && bp.buffer.Len() == 0 {
		return 0, io.EOF
	}

	return bp.buffer.Read(p)
}

func (bp *BufferedPipe) Close() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.closed = true
	bp.dataCond.Broadcast()
	return nil
}

// BufferedConnPipe creates a synchronous, in-memory, full duplex network connection.
// Both ends implement the net.Conn interface.
func BufferedConnPipe(block bool) (net.Conn, net.Conn) {
	p1 := NewBufferedPipe(block)
	p2 := NewBufferedPipe(block)

	c1 := &pipeConn{
		reader: p1,
		writer: p2,
	}
	c2 := &pipeConn{
		reader: p2,
		writer: p1,
	}

	return c1, c2
}

type pipeConn struct {
	reader *BufferedPipe
	writer *BufferedPipe
}

func (c *pipeConn) Read(b []byte) (n int, err error) {
	return c.reader.Read(b)
}

func (c *pipeConn) Write(b []byte) (n int, err error) {
	return c.writer.Write(b)
}

func (c *pipeConn) Close() error {
	err1 := c.reader.Close()
	err2 := c.writer.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func (c *pipeConn) LocalAddr() net.Addr {
	return pipeAddr{}
}

func (c *pipeConn) RemoteAddr() net.Addr {
	return pipeAddr{}
}

func (c *pipeConn) SetDeadline(t time.Time) error {
	return nil // No-op for now
}

func (c *pipeConn) SetReadDeadline(t time.Time) error {
	return nil // No-op for now
}

func (c *pipeConn) SetWriteDeadline(t time.Time) error {
	return nil // No-op for now
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }
