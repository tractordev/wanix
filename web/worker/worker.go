package worker

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
	resources map[string]fs.FS
	nextID    int
}

func New() *Device {
	return &Device{
		resources: make(map[string]fs.FS),
		nextID:    0,
	}
}

func (d *Device) Sub(name string) (fs.FS, error) {
	return fs.Sub(fskit.UnionFS{fskit.MapFS{
		"new": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			if name == "." {
				return &fskit.FuncFile{
					Node: fskit.Entry(name, 0555),
					ReadFunc: func(n *fskit.Node) error {
						d.nextID++
						rid := strconv.Itoa(d.nextID)
						d.resources[rid] = &Resource{
							id:    d.nextID,
							state: "allocated",
							src:   "",
						}
						fskit.SetData(n, []byte(rid+"\n"))
						return nil
					},
				}, nil
			}
			return nil, fs.ErrNotExist
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
	id    int
	state string
	src   string
}

func (r *Resource) Sub(name string) (fs.FS, error) {
	fsys := fskit.MapFS{
		"ctl": misc.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the worker",
			Run: func(ctx *cli.Context, args []string) {
				switch args[0] {
				case "start":
					r.state = "running"
					log.Println("start")
				case "terminate":
					r.state = "terminated"
					log.Println("terminate")
				}
			},
		}),
		"state": misc.FieldFile(r.state),
		"src": misc.FieldFile(r.src, func(data []byte) error {
			r.src = string(data)
			return nil
		}),
		// "err": misc.FieldFile(r.state, nil),
		// "fsys": misc.FieldFile(r.fs, nil),
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
