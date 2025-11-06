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
	"tractor.dev/wanix/internal"
	"tractor.dev/wanix/task"
	"tractor.dev/wanix/web/jsutil"
	"tractor.dev/wanix/web/runtime"
)

type Resource struct {
	id     int
	state  string
	src    string
	worker js.Value
	task   *task.Resource
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

	sys := runtime.Instance().Call("createPort")

	r.worker.Call("postMessage", map[string]any{"worker": map[string]any{
		"id":  r.id,
		"sys": sys,
		"cmd": strings.Join(args, " "),
		"env": env,
		"url": url,
	}}, []any{sys})

	r.state = "running"
	return nil
}

func (r *Resource) Open(name string) (fs.File, error) {
	return r.OpenContext(context.Background(), name)
}

func (r *Resource) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, rname, err := r.ResolveFS(ctx, name)
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, rname)
}

func (r *Resource) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	fsys := fskit.MapFS{
		"ctl": internal.ControlFile(&cli.Command{
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
		"state": internal.FieldFile(r.state),
		"src": internal.FieldFile(r.src, func(data []byte) error {
			r.src = string(data)
			return nil
		}),
	}
	return fs.Resolve(fsys, ctx, name)
}
