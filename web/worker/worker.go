//go:build js && wasm

package worker

import (
	"context"
	"log"
	"path"
	"strconv"
	"strings"
	"syscall/js"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/metacache"
	"tractor.dev/wanix/fs/p9kit"
	"tractor.dev/wanix/misc"
	"tractor.dev/wanix/misc/jsutil"
	"tractor.dev/wanix/vm"
	"tractor.dev/wanix/web/sys"
)

type Resource struct {
	id     int
	state  string
	src    string
	worker js.Value
	task   *wanix.Task
}

type guestSetter interface {
	fs.FS
	SetGuest(fs.FS)
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
	r.worker.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
		js.Global().Get("console").Call("error", args[0])
		// load the script again in a way that will show an import/syntax error
		jsutil.LoadScript(url.String(), true)
		return nil
	}))
	r.worker.Set("onmessageerror", js.FuncOf(func(this js.Value, args []js.Value) any {
		js.Global().Get("console").Call("error", args[0])
		return nil
	}))

	wanix.SetWorker(r.task, r.worker)

	port := sys.Element().Call("_openPort", r.task.ID())
	p9 := sys.Element().Call("_open9P", r.task.ID())

	r.worker.Call("addEventListener", "message", js.FuncOf(func(this js.Value, args []js.Value) any {
		go func() {
			// all we handle are ns exports for now
			exportPort := args[0].Get("data").Get("export")
			if exportPort.IsUndefined() {
				return
			}
			exportPort.Set("onmessage", js.FuncOf(func(this js.Value, _ []js.Value) any {
				go func() {
					// use initial signal message to mount export
					conn := misc.NewFakeConn(jsutil.NewPortReadWriter(exportPort))
					exportFS, err := p9kit.ClientFS(conn, "")
					if err != nil {
						log.Println("error creating client for export", err)
						return
					}
					wanix.Export(r.task, exportFS)

					// vm guest is still special cased for now
					if args[0].Get("data").Get("vm").IsUndefined() {
						return
					}
					vmID := args[0].Get("data").Get("vm").String()
					rfsys, _, err := fs.Resolve(r.task.Root().NS(), context.Background(), path.Join("#vm", vmID))
					if err != nil {
						log.Println("error resolving vm", vmID, err)
						return
					}
					vms := rfsys.(*vm.Device)
					vm, err := vms.Lookup(vmID)
					if err != nil {
						log.Println("error looking up vm", vmID, err)
						return
					}
					log.Println("mounting guest...")
					if err := vm.SetGuest(metacache.New(exportFS)); err != nil {
						log.Println("error setting guest", err)
					}
				}()
				return nil
			}))

		}()

		return nil
	}))

	r.worker.Call("postMessage", map[string]any{"worker": map[string]any{
		"id":   r.id,
		"tid":  r.task.ID(),
		"port": port,
		"p9":   p9,
		"cmd":  strings.Join(args, " "),
		"env":  env,
		"url":  url,
	}}, []any{port, p9})

	r.state = "running"
	return nil
}

func (r *Resource) Open(name string) (fs.File, error) {
	return r.OpenContext(context.Background(), name)
}

func (r *Resource) OpenContext(ctx context.Context, name string) (fs.File, error) {
	return fs.OpenContext(ctx, r.rootFS(), name)
}

func (r *Resource) rootFS() fskit.MapFS {
	return fskit.MapFS{
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
	}
}

func (r *Resource) Route(ctx context.Context, name string) (fs.FS, string, error) {
	return r.rootFS().Route(ctx, name)
}
