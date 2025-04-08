//go:build js && wasm

package worker

import (
	"context"
	"strconv"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/task"
)

type Service struct {
	resources map[string]fs.FS
	nextID    int
	task      *task.Resource
}

func New(task *task.Resource) *Service {
	return &Service{
		resources: make(map[string]fs.FS),
		nextID:    0,
		task:      task,
	}
}

func (d *Service) Open(name string) (fs.File, error) {
	return d.OpenContext(context.Background(), name)
}

func (d *Service) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, rname, err := d.ResolveFS(ctx, name)
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, rname)
}

func (d *Service) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	return fs.Resolve(fskit.UnionFS{
		fskit.MapFS{
			"new": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
				if name == "." {
					return &fskit.FuncFile{
						Node: fskit.Entry(name, 0555),
						ReadFunc: func(n *fskit.Node) error {
							r, err := d.Alloc(d.task)
							if err != nil {
								return err
							}
							fskit.SetData(n, []byte(r.ID()+"\n"))
							return nil
						},
					}, nil
				}
				return nil, fs.ErrNotExist
			}),
		},
		fskit.MapFS(d.resources),
	}, ctx, name)
}

func (d *Service) Alloc(t *task.Resource) (*Resource, error) {
	if t == nil {
		t = d.task
	}
	d.nextID++
	rid := strconv.Itoa(d.nextID)
	r := &Resource{
		id:    d.nextID,
		state: "allocated",
		src:   "",
		task:  t,
	}
	d.resources[rid] = r
	return r, nil
}
