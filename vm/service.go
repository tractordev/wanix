//go:build js && wasm

package vm

import (
	"context"
	"strconv"

	"tractor.dev/wanix"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	v86 "tractor.dev/wanix/vm/v86"
)

type Service struct {
	kinds     map[string]wanix.Factory
	resources map[string]fs.FS
	nextID    int
}

func New() *Service {
	d := &Service{
		kinds:     make(map[string]wanix.Factory),
		resources: make(map[string]fs.FS),
		nextID:    0,
	}
	// always register v86 for now
	d.Register("default", v86.New)
	return d
}

func (d *Service) Register(kind string, factory wanix.Factory) {
	d.kinds[kind] = factory
}

func (d *Service) Alloc(kind string) (wanix.Resource, error) {
	factory, ok := d.kinds[kind]
	if !ok {
		return nil, fs.ErrNotExist
	}
	d.nextID++
	rid := strconv.Itoa(d.nextID)
	r := factory(rid, kind)
	d.resources[rid] = r
	return r, nil
}

func (d *Service) Open(name string) (fs.File, error) {
	return d.OpenContext(context.Background(), name)
}

func (d *Service) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys := fskit.MapFS{
		"new": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			if name == "." {
				var nodes []fs.DirEntry
				for kind := range d.kinds {
					nodes = append(nodes, fskit.Entry(kind, 0555))
				}
				return fskit.DirFile(fskit.Entry("new", 0555), nodes...), nil
			}
			return &fskit.FuncFile{
				Node: fskit.Entry(name, 0555),
				ReadFunc: func(n *fskit.Node) error {
					r, err := d.Alloc(name)
					if err != nil {
						return err
					}
					fskit.SetData(n, []byte(r.ID()+"\n"))
					return nil
				},
			}, nil
		}),
	}
	return fs.OpenContext(ctx, fskit.UnionFS{fsys, fskit.MapFS(d.resources)}, name)
}
