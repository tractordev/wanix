package term

import (
	"context"
	"strconv"
	"sync"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/pipe"
	"tractor.dev/wanix/fs/signal"
)

type programFile struct {
	*pipe.PortFile
	prev byte // last input byte seen, for cross-call lookbehind
}

func (c *programFile) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	buf := make([]byte, 0, len(p)+len(p)/16)
	prev := c.prev
	for _, b := range p {
		if b == '\n' && prev != '\r' {
			buf = append(buf, '\r')
		}
		buf = append(buf, b)
		prev = b
	}
	if _, err := c.PortFile.Write(buf); err != nil {
		return 0, err
	}
	c.prev = prev
	return len(p), nil
}

// Resource is one terminal instance (paths data, program, winch).
// MapFS is embedded so ResolveFS reaches the signal FS (for fs.OpenFile with O_WRONLY, etc.).
type Resource struct {
	fskit.MapFS
	hub *signal.Broadcaster
	end *pipe.Port
}

func (r *Resource) shutdown() {
	r.hub.Close()
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

	// force xterm css to load if not already
	loadXtermCSS()

	d.nextID++
	rid = strconv.Itoa(d.nextID)
	hub := signal.NewBroadcaster()
	_, dataPF, progPF := pipe.NewFS(true)
	// remove := func() {
	// 	d.remove(rid)
	// }
	progWrap := &programFile{PortFile: progPF}
	root := fskit.MapFS{
		"data":    fskit.FileFS(dataPF, "data"),
		"program": fskit.FileFS(progWrap, "program"),
		"winch":   signal.NewFS(hub),
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
