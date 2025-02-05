package misc

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"strings"
	"sync"

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
}

func NewBufferedPipe() *BufferedPipe {
	bp := &BufferedPipe{}
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

	for bp.buffer.Len() == 0 && !bp.closed {
		bp.dataCond.Wait()
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
