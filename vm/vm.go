package vm

import (
	"context"
	"strings"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/misc"
)

type VM struct {
	id     string
	alias  string
	kind   string
	device *Device
}

func (r *VM) ID() string {
	return r.id
}

func (r *VM) Alias() string {
	return r.alias
}

func (r *VM) Kind() string {
	return r.kind
}

func (r *VM) Open(name string) (fs.File, error) {
	return r.OpenContext(context.Background(), name)
}

func (r *VM) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys := fskit.MapFS{
		"ctl": misc.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the resource",
			Run: func(_ *cli.Context, args []string) {
				switch args[0] {
				case "start":
					// todo: check if already started
					// options, err := parseFlags(args[1:])
					// if err != nil {
					// 	log.Println("vm start:", err)
					// 	return
					// }
					// serialport, screenport, shmpipe := makeVM(r.ID(), options, false)
					// r.screenport = screenport
					// r.shmpipe = shmpipe
					// r.serial = newSerialReadWriter(serialport)
					// if err := TryPatch(ctx, r.serial, serialFile); err != nil {
					// 	log.Println("vm start:", err)
					// }
					// fsys, _, ok := fs.Origin(ctx)
					// if ok {
					// 	cachedfs := metacache.New(fsys)
					// 	srv := p9.NewServer(p9kit.Attacher(cachedfs, p9kit.WithMemAttrStore()), p9.WithServerLogger(ulog.Log))
					// 	log.Println("starting 9p server for shmpipe")
					// 	go srv.Handle(shmpipe, shmpipe)
					// }
				}
			},
		}),
		"id":   misc.FieldFile(r.ID()),
		"kind": misc.FieldFile(r.Kind()),
		"alias": misc.FieldFile(r.alias, func(in []byte) error {
			if len(in) > 0 {
				oldalias := r.alias
				r.alias = strings.TrimSpace(string(in))
				r.device.mu.Lock()
				if oldalias != "" {
					delete(r.device.aliases, oldalias)
				}
				r.device.aliases[r.alias] = r
				r.device.mu.Unlock()
			}
			return nil
		}),
	}
	// if r.serial != nil {
	// 	fsys["ttyS0"] = fskit.FileFS(serialFile, "ttyS0")
	// }
	// if r.shmpipe != nil {
	// 	fsys["shmpipe0"] = fskit.FileFS(shmpipeFile, "shmpipe0")
	// }
	return fs.OpenContext(ctx, fsys, name)
}
