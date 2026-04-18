package wanix

import (
	"context"
	"log"
	"strconv"
	"strings"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/pipe"
	"tractor.dev/wanix/fs/vfs"
	"tractor.dev/wanix/internal"
)

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation.
type contextKey struct {
	name string
}

func (k *contextKey) String() string { return "task context value " + k.name }

var (
	TaskContextKey = &contextKey{"task"}
)

func FromContext(ctx context.Context) (*Task, bool) {
	p, ok := ctx.Value(TaskContextKey).(*Task)
	return p, ok
}

type Task struct {
	starter func(*Task) error
	parent  *Task
	ns      *vfs.NS
	id      int
	typ     string
	cmd     string
	env     []string
	exit    string
	dir     string
	fds     map[string]fs.FS
	sys     map[string]fs.FS
	closer  func()
	fsys    *TaskFS
}

// kludge: this would imply task specific registration, but its global.
// this is until we have a better registration system.
func (r *Task) Register(kind string, starter func(*Task) error) {
	r.fsys.types[kind] = starter
}

func (r *Task) Start() error {
	if r.starter != nil {
		return r.starter(r)
	}
	return nil
}

func (r *Task) ID() string {
	return strconv.Itoa(r.id)
}

func (r *Task) Context() context.Context {
	return r.Namespace().Context()
}

func (r *Task) Namespace() *vfs.NS {
	return r.ns
}

func (r *Task) Cmd() string {
	return r.cmd
}

func (r *Task) Env() []string {
	return r.env
}

func (r *Task) Dir() string {
	return r.dir
}

func (r *Task) Bind(srcPath, dstPath string) error {
	return r.ns.Bind(r.ns, srcPath, dstPath)
}

func (r *Task) Unbind(srcPath, dstPath string) error {
	return r.ns.Unbind(r.ns, srcPath, dstPath)
}

func (r *Task) Open(name string) (fs.File, error) {
	return r.OpenContext(context.Background(), name)
}

func (r *Task) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	return fs.Resolve(fskit.MapFS{
		"ctl": internal.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the Task",
			Run: func(ctx *cli.Context, args []string) {
				if len(args) == 3 && args[0] == "bind" {
					if err := r.Bind(args[1], args[2]); err != nil {
						log.Println(err)
					}
					return
				}
				if len(args) == 3 && args[0] == "unbind" {
					if err := r.Unbind(args[1], args[2]); err != nil {
						log.Println(err)
					}
					return
				}
				if len(args) == 1 && args[0] == "start" {
					if err := r.Start(); err != nil {
						log.Println(err)
					}
					return
				}
			},
		}),
		"type": internal.FieldFile(r.typ),
		"cmd": internal.FieldFile(r.cmd, func(in []byte) error {
			if len(in) > 0 {
				r.cmd = strings.TrimSpace(string(in))
			}
			return nil
		}),
		"env": internal.FieldFile(strings.Join(r.env, "\n"), func(in []byte) error {
			if len(in) > 0 {
				r.env = strings.Split(strings.TrimSpace(string(in)), "\n")
			}
			return nil
		}),
		"dir": internal.FieldFile(r.dir, func(in []byte) error {
			if len(in) > 0 {
				r.dir = strings.TrimSpace(string(in))
			}
			return nil
		}),
		"exit": internal.FieldFile(r.exit, func(in []byte) error {
			if len(in) > 0 {
				r.exit = strings.TrimSpace(string(in))
				if r.closer != nil {
					go r.closer()
				}
			}
			return nil
		}),
		"ns":   r.ns,
		"fd":   fskit.MapFS(r.fds),
		".sys": fskit.MapFS(r.sys),
	}, ctx, name)
}

func (r *Task) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, rname, err := r.ResolveFS(ctx, name)
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, rname)
}

type TaskFS struct {
	types     map[string]func(*Task) error
	resources map[string]fs.FS
	nextID    int
}

func NewTaskFS() *TaskFS {
	d := &TaskFS{
		types:     make(map[string]func(*Task) error),
		resources: make(map[string]fs.FS),
		nextID:    0,
	}
	// empty namespace process
	d.Register("", func(_ *Task) error {
		return nil
	})
	return d
}

func (d *TaskFS) Register(kind string, starter func(*Task) error) {
	d.types[kind] = starter
}

func (d *TaskFS) Alloc(kind string, parent *Task) (*Task, error) {
	starter, ok := d.types[kind]
	if !ok {
		return nil, fs.ErrNotExist
	}
	d.nextID++
	rid := strconv.Itoa(d.nextID)

	_, p0a, p0b := pipe.NewFS(false)
	_, p1a, p1b := pipe.NewFS(false)
	_, p2a, p2b := pipe.NewFS(false)

	p := &Task{
		fsys:    d,
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

func (d *TaskFS) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
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

func (d *TaskFS) Stat(name string) (fs.FileInfo, error) {
	log.Println("bare stat:", name)
	return d.StatContext(context.Background(), name)
}

func (d *TaskFS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	fsys, rname, err := d.ResolveFS(ctx, name)
	if err != nil {
		return nil, err
	}
	return fs.StatContext(ctx, fsys, rname)
}

func (d *TaskFS) Open(name string) (fs.File, error) {
	log.Println("bare open:", name)
	return d.OpenContext(context.Background(), name)
}

func (d *TaskFS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, rname, err := d.ResolveFS(ctx, name)
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, rname)
}
