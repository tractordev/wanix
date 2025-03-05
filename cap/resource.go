package cap

import (
	"context"
	"log"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/misc"
)

type Resource struct {
	mounter Mounter
	fs      fs.FS
	id      int
	typ     string
	extra   map[string]fs.FS
}

func (r *Resource) Sub(name string) (fs.FS, error) {
	fsys := fskit.MapFS{
		"ctl": misc.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the resource",
			Run: func(ctx *cli.Context, args []string) {
				if args[0] == "mount" {
					var err error
					r.fs, err = r.mounter(args[1:])
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
	for k, v := range r.extra {
		fsys[k] = v
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
