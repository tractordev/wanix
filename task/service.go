package task

import (
	"context"
	"log"
	"strconv"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/internal"
	"tractor.dev/wanix/vfs"
	"tractor.dev/wanix/vfs/pipe"
)

type Service struct {
	types     map[string]func(*Resource) error
	resources map[string]fs.FS
	nextID    int
}

func New() *Service {
	d := &Service{
		types:     make(map[string]func(*Resource) error),
		resources: make(map[string]fs.FS),
		nextID:    0,
	}
	// empty namespace process
	d.Register("ns", func(_ *Resource) error {
		return nil
	})
	return d
}

func (d *Service) Register(kind string, starter func(*Resource) error) {
	d.types[kind] = starter
}

func (d *Service) Alloc(kind string, parent *Resource) (*Resource, error) {
	starter, ok := d.types[kind]
	if !ok {
		return nil, fs.ErrNotExist
	}
	d.nextID++
	rid := strconv.Itoa(d.nextID)

	_, p0a, p0b := pipe.NewFS(false)
	_, p1a, p1b := pipe.NewFS(false)
	_, p2a, p2b := pipe.NewFS(false)

	p := &Resource{
		starter: starter,
		id:      d.nextID,
		typ:     kind,
		fds: map[string]fs.FS{
			"0": fskit.FileFS(p0a, "0"),
			"1": fskit.FileFS(p1a, "1"),
			"2": fskit.FileFS(p2a, "2"),
		},
		sys: map[string]fs.FS{
			"fd0": fskit.FileFS(p0b, "fd0"),
			"fd1": fskit.FileFS(p1b, "fd1"),
			"fd2": fskit.FileFS(p2b, "fd2"),
		},
		closer: func() {
			p0b.Port.Close()
			p1b.Port.Close()
			p2b.Port.Close()
		},
	}
	ctx := context.WithValue(context.Background(), TaskContextKey, p)
	if parent != nil {
		p.parent = parent
		p.ns = parent.ns.Clone(ctx)
	} else {
		p.ns = vfs.New(ctx)
	}
	d.resources[rid] = p
	return p, nil
}

func (d *Service) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	m := fskit.MapFS{
		"new": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			if name == "." {
				var nodes []fs.DirEntry
				for kind := range d.types {
					nodes = append(nodes, fskit.Entry(kind, 0555))
				}
				return fskit.DirFile(fskit.Entry("new", 0555), nodes...), nil
			}
			return &fskit.FuncFile{
				Node: fskit.Entry(name, 0555),
				ReadFunc: func(n *fskit.Node) error {
					t, _ := FromContext(ctx)
					p, err := d.Alloc(name, t)
					if err != nil {
						return err
					}
					fskit.SetData(n, []byte(p.ID()+"\n"))
					return nil
				},
			}, nil
		}),
	}
	t, ok := FromContext(ctx)
	if ok {
		m["self"] = internal.FieldFile(t.ID(), nil)
	}
	return fs.Resolve(fskit.UnionFS{m, fskit.MapFS(d.resources)}, ctx, name)
}

func (d *Service) Stat(name string) (fs.FileInfo, error) {
	log.Println("bare stat:", name)
	return d.StatContext(context.Background(), name)
}

func (d *Service) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	fsys, rname, err := d.ResolveFS(ctx, name)
	if err != nil {
		return nil, err
	}
	return fs.StatContext(ctx, fsys, rname)
}

func (d *Service) Open(name string) (fs.File, error) {
	log.Println("bare open:", name)
	return d.OpenContext(context.Background(), name)
}

func (d *Service) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, rname, err := d.ResolveFS(ctx, name)
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, rname)
}
