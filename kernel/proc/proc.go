package proc

import (
	"context"
	"fmt"
	"strconv"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/kernel/ns"
	"tractor.dev/wanix/kernel/p9"
)

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation.
type contextKey struct {
	name string
}

func (k *contextKey) String() string { return "wanix/kernel context value " + k.name }

var (
	ProcessContextKey = &contextKey{"process"}
)

func NewContextWithPID(ctx context.Context, pid string) context.Context {
	return context.WithValue(ctx, ProcessContextKey, pid)
}

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
	return &Device{
		types:     make(map[string]func([]string) (fs.FS, error)),
		resources: make(map[string]fs.FS),
		nextID:    0,
	}
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
	p := &Process{
		id:      d.nextID,
		typ:     kind,
		factory: factory,
		fs:      ns.New(),
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
		fsys["self"] = p9.FieldFile(pid, nil)
	}
	return fs.OpenContext(ctx, fskit.UnionFS{fsys, fskit.MapFS(d.resources)}, name)
}

type Process struct {
	factory func([]string) (fs.FS, error)
	fs      *ns.FS
	id      int
	typ     string
}

func (r *Process) ID() string {
	return strconv.Itoa(r.id)
}

func (r *Process) Namespace() *ns.FS {
	return r.fs
}

func (r *Process) Open(name string) (fs.File, error) {
	return r.OpenContext(context.Background(), name)
}

func (r *Process) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys := fskit.MapFS{
		"ctl": p9.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the resource",
			Run: func(ctx *cli.Context, args []string) {
				fmt.Println("proc ctl:", args)
			},
		}),
		"type": p9.FieldFile(r.typ, nil),
		"ns":   r.fs,
	}
	return fs.OpenContext(ctx, fsys, name)
}
