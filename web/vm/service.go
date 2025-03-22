//go:build js && wasm

package vm

import (
	"context"
	"strconv"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

type Service struct {
	resources map[string]fs.FS
	nextID    int
}

func New() *Service {
	d := &Service{
		resources: make(map[string]fs.FS),
		nextID:    0,
	}
	return d
}

func (d *Service) Open(name string) (fs.File, error) {
	return d.OpenContext(context.Background(), name)
}

func (d *Service) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys := fskit.MapFS{
		"new": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			if name == "." {
				return &fskit.FuncFile{
					Node: fskit.Entry(name, 0555),
					ReadFunc: func(n *fskit.Node) error {
						d.nextID++
						rid := strconv.Itoa(d.nextID)
						vm := makeVM()
						d.resources[rid] = &VM{
							id:     d.nextID,
							typ:    name,
							value:  vm,
							serial: newSerial(vm),
						}
						fskit.SetData(n, []byte(rid+"\n"))
						return nil
					},
				}, nil
			}
			return nil, fs.ErrNotExist
		}),
	}
	return fs.OpenContext(ctx, fskit.UnionFS{fsys, fskit.MapFS(d.resources)}, name)
}
