package proc

import (
	"context"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/kernel/ns"
	"tractor.dev/wanix/misc"
)

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation.
type contextKey struct {
	name string
}

func (k *contextKey) String() string { return "proc context value " + k.name }

var (
	ProcessContextKey = &contextKey{"process"}
)

func PIDFromContext(ctx context.Context) (string, bool) {
	p, ok := ctx.Value(ProcessContextKey).(string)
	return p, ok
}

type Device struct {
	types     map[string]func(*Process) error
	resources map[string]fs.FS
	nextID    int
}

func New() *Device {
	d := &Device{
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

func (d *Device) Register(kind string, starter func(*Process) error) {
	d.types[kind] = starter
}

func (d *Device) Alloc(kind string) (*Process, error) {
	starter, ok := d.types[kind]
	if !ok {
		return nil, fs.ErrNotExist
	}
	d.nextID++
	rid := strconv.Itoa(d.nextID)
	ctx := context.WithValue(context.Background(), ProcessContextKey, rid)
	a0, b0 := misc.BufferedConnPipe(false)
	a1, b1 := misc.BufferedConnPipe(false)
	a2, b2 := misc.BufferedConnPipe(false)
	p := &Process{
		starter: starter,
		id:      d.nextID,
		typ:     kind,
		ns:      ns.New(ctx),
		fds: map[string]fs.FS{
			"0": newFdFile(a0, "0"),
			"1": newFdFile(a1, "1"),
			"2": newFdFile(a2, "2"),
			// temporary until find a better way:
			"worker0": newFdFile(b0, "worker0"),
			"worker1": newFdFile(b1, "worker1"),
			"worker2": newFdFile(b2, "worker2"),
		},
	}
	d.resources[rid] = p
	return p, nil
}

func (d *Device) fsys() (fs.FS, fskit.MapFS) {
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

func (d *Device) Sub(name string) (fs.FS, error) {
	fsys, _ := d.fsys()
	return fs.Sub(fsys, name)
}

func (d *Device) Open(name string) (fs.File, error) {
	return d.OpenContext(context.Background(), name)
}

func (d *Device) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, dir := d.fsys()
	pid, ok := PIDFromContext(ctx)
	if ok {
		dir["self"] = misc.FieldFile(pid, nil)
	}
	return fs.OpenContext(ctx, fsys, name)
}

type Process struct {
	starter func(*Process) error
	ns      *ns.FS
	id      int
	typ     string
	cmd     string
	env     []string
	exit    string
	dir     string
	fds     map[string]fs.FS
}

func (r *Process) Start() error {
	if r.starter != nil {
		return r.starter(r)
	}
	return nil
}

func (r *Process) ID() string {
	return strconv.Itoa(r.id)
}

func (r *Process) Context() context.Context {
	return r.Namespace().Context()
}

func (r *Process) Namespace() *ns.FS {
	return r.ns
}

func (r *Process) Cmd() string {
	return r.cmd
}

func (r *Process) Env() []string {
	return r.env
}

func (r *Process) Dir() string {
	return r.dir
}

func (r *Process) Bind(srcPath, dstPath string) error {
	return r.ns.Bind(r.ns, srcPath, dstPath, "")
}

func (r *Process) Open(name string) (fs.File, error) {
	return r.OpenContext(context.Background(), name)
}

func (r *Process) Sub(name string) (fs.FS, error) {
	return fs.Sub(fskit.MapFS{
		"ctl": misc.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the resource",
			Run: func(ctx *cli.Context, args []string) {
				if len(args) == 3 && args[0] == "bind" {
					if err := r.Bind(args[1], args[2]); err != nil {
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
		"type": misc.FieldFile(r.typ),
		"cmd": misc.FieldFile(r.cmd, func(in []byte) error {
			if len(in) > 0 {
				r.cmd = strings.TrimSpace(string(in))
			}
			return nil
		}),
		"env": misc.FieldFile(strings.Join(r.env, "\n"), func(in []byte) error {
			if len(in) > 0 {
				r.env = strings.Split(strings.TrimSpace(string(in)), " ")
			}
			return nil
		}),
		"dir": misc.FieldFile(r.dir, func(in []byte) error {
			if len(in) > 0 {
				r.dir = strings.TrimSpace(string(in))
			}
			return nil
		}),
		"exit": misc.FieldFile(r.exit, func(in []byte) error {
			if len(in) > 0 {
				r.exit = strings.TrimSpace(string(in))
			}
			return nil
		}),
		"ns": r.ns,
		"fd": fskit.MapFS(r.fds),
	}, name)
}

func (r *Process) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, err := r.Sub(".")
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, name)
}

type ConnFile struct {
	net.Conn
	Name string
}

func newFdFile(conn net.Conn, name string) fs.FS {
	return fskit.OpenFunc(func(ctx context.Context, _ string) (fs.File, error) {
		return &ConnFile{Conn: conn, Name: name}, nil
	})
}

func (s *ConnFile) Stat() (fs.FileInfo, error) {
	return fskit.Entry(s.Name, 0644), nil
}

func (s *ConnFile) WriteAt(p []byte, off int64) (n int, err error) {
	return s.Write(p)
}

func (s *ConnFile) Write(p []byte) (n int, err error) {
	return s.Conn.Write(p)
}

func (s *ConnFile) ReadAt(p []byte, off int64) (n int, err error) {
	if off > 0 {
		return 0, io.EOF
	}
	return s.Read(p)
}

func (s *ConnFile) Read(p []byte) (int, error) {
	return s.Conn.Read(p)
}

func (s *ConnFile) Close() error {
	return nil
}
