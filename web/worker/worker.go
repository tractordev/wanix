//go:build js && wasm

package worker

import (
	"context"
	"strconv"
	"strings"
	"syscall/js"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/kernel"
	"tractor.dev/wanix/misc"
	"tractor.dev/wanix/web/api"
)

type Device struct {
	resources map[string]fs.FS
	nextID    int
	k         *kernel.K
}

func New(k *kernel.K) *Device {
	return &Device{
		resources: make(map[string]fs.FS),
		nextID:    0,
		k:         k,
	}
}

func (d *Device) Alloc() (*Resource, error) {
	d.nextID++
	rid := strconv.Itoa(d.nextID)
	r := &Resource{
		id:    d.nextID,
		state: "allocated",
		src:   "",
		k:     d.k,
	}
	d.resources[rid] = r
	return r, nil
}

func (d *Device) Sub(name string) (fs.FS, error) {
	return fs.Sub(fskit.UnionFS{fskit.MapFS{
		"new": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			if name == "." {
				return &fskit.FuncFile{
					Node: fskit.Entry(name, 0555),
					ReadFunc: func(n *fskit.Node) error {
						r, err := d.Alloc()
						if err != nil {
							return err
						}
						fskit.SetData(n, []byte(r.ID()+"\n"))
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
	id     int
	state  string
	src    string
	worker js.Value
	k      *kernel.K
}

func (r *Resource) ID() string {
	return strconv.Itoa(r.id)
}

func (r *Resource) Start(args ...string) error {
	env := make(map[string]any)

	// Parse environment variables from args
	argIndex := 0
	for ; argIndex < len(args); argIndex++ {
		if !strings.Contains(args[argIndex], "=") {
			break
		}
		parts := strings.SplitN(args[argIndex], "=", 2)
		env[parts[0]] = parts[1]
	}
	// Remove env vars from args
	if argIndex > 0 {
		args = args[argIndex:]
	}

	// use args[0] as the worker script
	// or use r.src if no args
	var url js.Value = js.Undefined()
	if len(args) > 0 {
		url = js.ValueOf(args[0])
	} else {
		blob := js.Global().Get("Blob").New(js.ValueOf([]any{r.src}), js.ValueOf(map[string]any{"type": "text/javascript"}))
		url = js.Global().Get("URL").Call("createObjectURL", blob)
	}

	r.worker = js.Global().Get("Worker").New(url, js.ValueOf(map[string]any{"type": "module"}))

	ch := js.Global().Get("MessageChannel").New()
	connPort := js.Global().Get("wanix").Call("_toport", ch.Get("port1"))
	go api.PortResponder(connPort, r.k.Root)

	r.worker.Call("postMessage", map[string]any{"worker": map[string]any{
		"id":      r.id,
		"fsys":    ch.Get("port2"),
		"cmdline": strings.Join(args, " "),
		"env":     env,
	}}, []any{ch.Get("port2")})

	r.state = "running"
	return nil
}

func (r *Resource) Sub(name string) (fs.FS, error) {
	fsys := fskit.MapFS{
		"ctl": misc.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the worker",
			Run: func(ctx *cli.Context, args []string) {
				switch args[0] {
				case "start":
					r.Start(args[1:]...)
				case "terminate":
					if !r.worker.IsUndefined() {
						r.worker.Call("terminate")
					}
					r.state = "terminated"
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
