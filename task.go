package wanix

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
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

type TaskDriver interface {
	Check(*Task) bool
	Start(*Task) error
}

type Task struct {
	driver TaskDriver
	parent *Task
	ns     *vfs.NS
	id     int
	alias  string
	typ    string
	cmd    string
	env    []string
	exit   string
	dir    string
	fds    map[int]*openFile
	fdIdx  int
	closer func()
	fsys   *TaskFS
	mu     sync.Mutex
}

type openFile struct {
	file fs.File
	path string
	// more?
}

// kludge: this would imply task specific registration, but its global.
// this is until we have a better registration system.
func (t *Task) Register(kind string, driver TaskDriver) {
	t.fsys.types[kind] = driver
}

func (t *Task) Lookup(rid string) (*Task, error) {
	t.fsys.mu.Lock()
	defer t.fsys.mu.Unlock()
	tt, ok := t.fsys.resources[rid]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return tt.(*Task), nil
}

func (t *Task) Start() error {
	if t.driver != nil {
		return t.driver.Start(t)
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

func (r *Task) Arg(idx int) string {
	args := strings.Split(r.cmd, " ")
	if idx < 0 || idx >= len(args) {
		return ""
	}
	return args[idx]
}

func (r *Task) Env() []string {
	return r.env
}

func (r *Task) Alias() string {
	return r.alias
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

func (r *Task) OpenFD(file fs.File, path string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fdIdx++
	r.fds[r.fdIdx] = &openFile{file: file, path: path}
	return r.fdIdx
}

func (r *Task) CloseFD(fd int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if fd < 0 || fd > r.fdIdx {
		return fs.ErrInvalid
	}
	f, ok := r.fds[fd]
	if !ok {
		return fs.ErrInvalid
	}
	delete(r.fds, fd)
	return f.file.Close()
}

func (r *Task) FD(fd int) (fs.File, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if fd < 0 || fd > r.fdIdx {
		return nil, "", fs.ErrInvalid
	}
	if fd < 3 {
		stdfile, err := r.Namespace().Open(fmt.Sprintf("#task/%s/fd/%d", r.ID(), fd))
		if err != nil {
			return nil, "", err
		}
		r.fds[fd] = &openFile{file: stdfile, path: fmt.Sprintf("#task/%s/fd/%d", r.ID(), fd)}
	}
	f, ok := r.fds[fd]
	if !ok {
		return nil, "", fs.ErrInvalid
	}
	return f.file, f.path, nil
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
		"id":   internal.FieldFile(r.ID()),
		"type": internal.FieldFile(r.typ),
		"cmd": internal.FieldFile(r.cmd, func(in []byte) error {
			if len(in) > 0 {
				r.cmd = strings.TrimSpace(string(in))
			}
			return nil
		}),
		"alias": internal.FieldFile(r.alias, func(in []byte) error {
			if len(in) > 0 {
				oldalias := r.alias
				r.alias = strings.TrimSpace(string(in))
				r.fsys.mu.Lock()
				if oldalias != "" {
					delete(r.fsys.aliases, oldalias)
				}
				r.fsys.aliases[r.alias] = r
				r.fsys.mu.Unlock()
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
		"ns": r.ns,
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
	types     map[string]TaskDriver
	resources map[string]fs.FS
	aliases   map[string]fs.FS
	nextID    int
	mu        sync.Mutex
}

type autoDriver func(*Task) error

func (d autoDriver) Check(*Task) bool {
	return false
}

func (d autoDriver) Start(t *Task) error {
	return d(t)
}

func NewTaskFS() *TaskFS {
	d := &TaskFS{
		types:     make(map[string]TaskDriver),
		resources: make(map[string]fs.FS),
		aliases:   make(map[string]fs.FS),
		nextID:    0,
	}
	// empty namespace process
	d.Register("auto", autoDriver(func(t *Task) error {
		d.mu.Lock()
		defer d.mu.Unlock()
		for kind, driver := range d.types {
			if driver.Check(t) {
				t.typ = kind
				return driver.Start(t)
			}
		}
		return nil
	}))
	return d
}

func (d *TaskFS) Register(kind string, driver TaskDriver) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.types[kind] = driver
}

func (d *TaskFS) Alloc(kind string, parent *Task) (*Task, error) {
	d.mu.Lock()
	driver, ok := d.types[kind]
	d.mu.Unlock()
	if !ok {
		return nil, fs.ErrNotExist
	}
	d.nextID++
	rid := strconv.Itoa(d.nextID)

	p := &Task{
		fsys:   d,
		driver: driver,
		id:     d.nextID,
		typ:    kind,
		fds:    make(map[int]*openFile),
		fdIdx:  3,
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
	fsys := vfs.New(ctx)
	if err := fsys.Bind(fskit.MapFS(d.aliases), ".", "."); err != nil {
		return nil, "", err
	}
	if err := fsys.Bind(fskit.MapFS(d.resources), ".", "."); err != nil {
		return nil, "", err
	}
	if err := fsys.Bind(m, ".", "."); err != nil {
		return nil, "", err
	}
	t, ok := FromContext(ctx)
	if ok {
		if err := fsys.Bind(d.resources[t.ID()], ".", "self"); err != nil {
			return nil, "", err
		}
	}
	return fs.Resolve(fsys, ctx, name)
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
