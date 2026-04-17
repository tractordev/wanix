package term

import (
	"context"
	"strconv"
	"sync"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/pipe"
)

type programFile struct {
	*pipe.PortFile
	once   sync.Once
	remove func()
}

// func (p *programFile) Read(b []byte) (int, error) {
// 	n, err := p.PortFile.Read(b)
// 	if errors.Is(err, io.EOF) {
// 		p.once.Do(p.remove)
// 	}
// 	return n, err
// }

// func (p *programFile) Close() error {
// 	log.Println("close program file")
// 	p.once.Do(p.remove)
// 	return p.PortFile.Close()
// }

// Resource is one terminal instance (paths data, program, winch).
// MapFS is embedded so ResolveFS reaches winchFS (for fs.OpenFile with O_WRONLY, etc.).
type Resource struct {
	fskit.MapFS
	hub *winchHub
	end *pipe.Port
}

func (r *Resource) shutdown() {
	r.hub.close()
	if r.end != nil {
		r.end.Close()
	}
}

type Service struct {
	mu        sync.RWMutex
	resources map[string]fs.FS
	nextID    int

	AllocHook func(s *Service, rid string) error
}

func New() *Service {
	return &Service{
		resources: make(map[string]fs.FS),
		nextID:    0,
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

func (d *Service) Stat(name string) (fs.FileInfo, error) {
	return d.StatContext(context.Background(), name)
}

func (d *Service) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	fsys, rname, err := d.ResolveFS(ctx, name)
	if err != nil {
		return nil, err
	}
	return fs.StatContext(ctx, fsys, rname)
}

func (d *Service) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	return fs.Resolve(fskit.UnionFS{
		fskit.MapFS{
			"new": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
				if name == "." {
					return &fskit.FuncFile{
						Node: fskit.Entry(name, 0555),
						ReadFunc: func(n *fskit.Node) error {
							rid, err := d.Alloc()
							if err != nil {
								return err
							}
							if d.AllocHook != nil {
								err = d.AllocHook(d, rid)
								if err != nil {
									return err
								}
							}
							fskit.SetData(n, []byte(rid+"\n"))
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

func (d *Service) Get(rid string) (*Resource, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	res, ok := d.resources[rid]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return res.(*Resource), nil
}

func (d *Service) Alloc() (rid string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nextID++
	rid = strconv.Itoa(d.nextID)
	hub := newWinchHub()
	_, dataPF, progPF := pipe.NewFS(true)
	remove := func() {
		d.remove(rid)
	}
	progWrap := &programFile{PortFile: progPF, remove: remove}
	root := fskit.MapFS{
		"data":    fskit.FileFS(dataPF, "data"),
		"program": fskit.FileFS(progWrap, "program"),
		"winch":   &winchFS{hub: hub},
	}
	d.resources[rid] = &Resource{
		MapFS: root,
		hub:   hub,
		end:   progPF.Port,
	}
	return rid, nil
}

func (d *Service) remove(rid string) {
	d.mu.Lock()
	res, err := d.Get(rid)
	if err != nil {
		d.mu.Unlock()
		return
	}
	delete(d.resources, rid)
	d.mu.Unlock()
	res.shutdown()
}
