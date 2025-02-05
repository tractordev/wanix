package proc

import (
	"context"
	"log"
	"strconv"

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
	types     map[string]func([]string) (fs.FS, error)
	resources map[string]fs.FS
	nextID    int
}

func New() *Device {
	d := &Device{
		types:     make(map[string]func([]string) (fs.FS, error)),
		resources: make(map[string]fs.FS),
		nextID:    0,
	}
	// empty namespace process
	d.Register("ns", func(args []string) (fs.FS, error) {
		return nil, nil
	})
	return d
}

func (d *Device) Register(kind string, factory func([]string) (fs.FS, error)) {
	d.types[kind] = factory
}

func (d *Device) Alloc(kind string) (*Process, error) {
	factory, ok := d.types[kind]
	if !ok {
		return nil, fs.ErrNotExist
	}
	d.nextID++
	rid := strconv.Itoa(d.nextID)
	ctx := context.WithValue(context.Background(), ProcessContextKey, rid)
	p := &Process{
		id:      d.nextID,
		typ:     kind,
		factory: factory,
		ns:      ns.New(ctx),
	}
	d.resources[rid] = p
	return p, nil
}

func (d *Device) Open(name string) (fs.File, error) {
	return d.OpenContext(context.Background(), name)
}

func (d *Device) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys := fskit.MapFS{
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
	pid, ok := PIDFromContext(ctx)
	if ok {
		fsys["self"] = misc.FieldFile(pid, nil)
	}
	return fs.OpenContext(ctx, fskit.UnionFS{fsys, fskit.MapFS(d.resources)}, name)
}

type Process struct {
	factory func([]string) (fs.FS, error)
	ns      *ns.FS
	id      int
	typ     string
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

func (r *Process) Bind(srcPath, dstPath string) error {
	return r.ns.Bind(r.ns, srcPath, dstPath, "")
}

func (r *Process) Open(name string) (fs.File, error) {
	return r.OpenContext(context.Background(), name)
}

func (r *Process) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys := fskit.MapFS{
		"ctl": misc.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the resource",
			Run: func(ctx *cli.Context, args []string) {
				if len(args) == 3 && args[0] == "bind" {
					if err := r.Bind(args[1], args[2]); err != nil {
						log.Println(err)
					}
				}
			},
		}),
		"type": misc.FieldFile(r.typ, nil),
		"ns":   r.ns,
	}
	return fs.OpenContext(ctx, fsys, name)
}
