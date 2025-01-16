package p9

import (
	"context"
	"io/fs"
	"strings"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs/fskit"
)

func FieldFile(value string, setter func([]byte) error) fs.FS {
	var mode fs.FileMode = 0755
	if setter == nil {
		mode = 0555
	}
	return fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
		return &fskit.FuncFile{
			Node: fskit.Entry(name, mode, []byte(value+"\n")),
			CloseFunc: func(n *fskit.Node) error {
				if setter != nil {
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
