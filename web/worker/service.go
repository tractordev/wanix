//go:build js && wasm

package worker

import (
	"context"
	"strconv"

	"tractor.dev/wanix"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

type Service struct {
	resources map[string]fs.FS
	nextID    int
	k         *wanix.K
}

func New(k *wanix.K) *Service {
	return &Service{
		resources: make(map[string]fs.FS),
		nextID:    0,
		k:         k,
	}
}

func (d *Service) Alloc() (*Resource, error) {
	d.nextID++
	rid := strconv.Itoa(d.nextID)
	r := &Resource{
		id:    d.nextID,
		state: "allocated",
		src:   "",
		k:     d.k,
	}
	d.resources[rid] = r
	return r, nil
}

func (d *Service) Sub(name string) (fs.FS, error) {
	return fs.Sub(fskit.UnionFS{fskit.MapFS{
		"new": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			if name == "." {
				return &fskit.FuncFile{
					Node: fskit.Entry(name, 0555),
					ReadFunc: func(n *fskit.Node) error {
						r, err := d.Alloc()
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
	}, fskit.MapFS(d.resources)}, name)
}

func (d *Service) Open(name string) (fs.File, error) {
	return d.OpenContext(context.Background(), name)
}

func (d *Service) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, err := d.Sub(".")
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, name)
}
