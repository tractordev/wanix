package fsys

import (
	"context"
	"log"
	"strconv"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/misc"
)

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

func (d *Device) Sub(name string) (fs.FS, error) {
	return fs.Sub(fskit.UnionFS{fskit.MapFS{
		"ctl": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			return fskit.Entry(name, 0555, []byte("ctl\n")).Open(".")
		}),
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
					factory, ok := d.types[name]
					if !ok {
						return fs.ErrNotExist
					}
					d.nextID++
					rid := strconv.Itoa(d.nextID)
					d.resources[rid] = &Resource{
						id:      d.nextID,
						typ:     name,
						factory: factory,
					}
					fskit.SetData(n, []byte(rid+"\n"))
					return nil
				},
			}, nil
		}),
	}, fskit.MapFS(d.resources)}, name)
}

func (d *Device) Open(name string) (fs.File, error) {
	return d.OpenContext(context.Background(), name)
}

func (d *Device) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, err := d.Sub(".")
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, name)
}

type Resource struct {
	factory func([]string) (fs.FS, error)
	fs      fs.FS
	id      int
	typ     string
}

func (r *Resource) Sub(name string) (fs.FS, error) {
	fsys := fskit.MapFS{
		"ctl": misc.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the resource",
			Run: func(ctx *cli.Context, args []string) {
				if args[0] == "mount" {
					var err error
					r.fs, err = r.factory(args[1:])
					if err != nil {
						log.Println(err)
					}
				}
			},
		}),
		"type": misc.FieldFile(r.typ, nil),
	}
	if r.fs != nil {
		fsys["mount"] = r.fs
	}
	return fs.Sub(fsys, name)
}

func (r *Resource) Open(name string) (fs.File, error) {
	return r.OpenContext(context.Background(), name)
}

func (r *Resource) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, err := r.Sub(".")
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, name)
}
