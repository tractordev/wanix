package task

import (
	"context"
	"strconv"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/misc"
	"tractor.dev/wanix/namespace"
)

type Service struct {
	types     map[string]func(*Process) error
	resources map[string]fs.FS
	nextID    int
}

func New() *Service {
	d := &Service{
		types:     make(map[string]func(*Process) error),
		resources: make(map[string]fs.FS),
		nextID:    0,
	}
	// empty namespace process
	d.Register("ns", func(_ *Process) error {
		return nil
	})
	return d
}

func (d *Service) Register(kind string, starter func(*Process) error) {
	d.types[kind] = starter
}

func (d *Service) Alloc(kind string) (*Process, error) {
	starter, ok := d.types[kind]
	if !ok {
		return nil, fs.ErrNotExist
	}
	d.nextID++
	rid := strconv.Itoa(d.nextID)
	ctx := context.WithValue(context.Background(), TaskContextKey, rid)
	a0, b0 := misc.BufferedConnPipe(false)
	a1, b1 := misc.BufferedConnPipe(false)
	a2, b2 := misc.BufferedConnPipe(false)
	p := &Process{
		starter: starter,
		id:      d.nextID,
		typ:     kind,
		ns:      namespace.New(ctx),
		fds: map[string]fs.FS{
			"0": newFdFile(a0, "0"),
			"1": newFdFile(a1, "1"),
			"2": newFdFile(a2, "2"),
		},
		sys: map[string]fs.FS{
			"fd0": newFdFile(b0, "fd0"),
			"fd1": newFdFile(b1, "fd1"),
			"fd2": newFdFile(b2, "fd2"),
		},
	}
	d.resources[rid] = p
	return p, nil
}

func (d *Service) fsys() (fs.FS, fskit.MapFS) {
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
					p, err := d.Alloc(name)
					if err != nil {
						return err
					}
					fskit.SetData(n, []byte(p.ID()+"\n"))
					return nil
				},
			}, nil
		}),
	}
	return fskit.UnionFS{m, fskit.MapFS(d.resources)}, m
}

func (d *Service) Sub(name string) (fs.FS, error) {
	fsys, _ := d.fsys()
	return fs.Sub(fsys, name)
}

func (d *Service) Open(name string) (fs.File, error) {
	return d.OpenContext(context.Background(), name)
}

func (d *Service) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, dir := d.fsys()
	pid, ok := PIDFromContext(ctx)
	if ok {
		dir["self"] = misc.FieldFile(pid, nil)
	}
	return fs.OpenContext(ctx, fsys, name)
}
