package cap

import (
	"context"
	"strconv"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/memfs"
	"tractor.dev/wanix/vfs"
)

type Allocator func(*Resource) (Mounter, error)
type Mounter func([]string) (fs.FS, error)

type Service struct {
	allocators map[string]Allocator
	resources  map[string]fs.FS
	nextID     int
}

func New(nsch <-chan *vfs.NS) *Service {
	return &Service{
		allocators: map[string]Allocator{
			"loopback": loopbackAllocator(),
			"tarfs":    tarfsAllocator(),
			"tmpfs": func(r *Resource) (Mounter, error) {
				return func(_ []string) (fs.FS, error) {
					return memfs.New(), nil
				}, nil
			},
		},
		resources: make(map[string]fs.FS),
		nextID:    0,
	}
}

func (d *Service) Register(kind string, alloc Allocator) {
	d.allocators[kind] = alloc
}

func (d *Service) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	return fs.Resolve(fskit.UnionFS{fskit.MapFS{
		"ctl": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			return fskit.Entry(name, 0555, []byte("ctl\n")).Open(".")
		}),
		"new": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			if name == "." {
				var nodes []fs.DirEntry
				for kind := range d.allocators {
					nodes = append(nodes, fskit.Entry(kind, 0555))
				}
				return fskit.DirFile(fskit.Entry("new", 0555), nodes...), nil
			}
			return &fskit.FuncFile{
				Node: fskit.Entry(name, 0555),
				ReadFunc: func(n *fskit.Node) error {
					alloc, ok := d.allocators[name]
					if !ok {
						return fs.ErrNotExist
					}
					d.nextID++
					rid := strconv.Itoa(d.nextID)
					r := &Resource{
						id:      d.nextID,
						typ:     name,
						mounter: nil,
						Extra:   map[string]fs.FS{},
					}
					mounter, err := alloc(r)
					if err != nil {
						return err
					}
					r.mounter = mounter
					d.resources[rid] = r
					fskit.SetData(n, []byte(rid+"\n"))
					return nil
				},
			}, nil
		}),
	}, fskit.MapFS(d.resources)}, ctx, name)
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
