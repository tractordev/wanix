package internal

import (
	"context"
	"io/fs"
	"strings"
	"sync/atomic"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/internal/shlex"
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
