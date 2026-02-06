//go:build js && wasm

package v86

import (
	"context"
	"fmt"
	"io"
	"log"
	"path"
	"syscall/js"

	_ "embed"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/internal"
	"tractor.dev/wanix/vfs/pipe"
	"tractor.dev/wanix/web/jsutil"
	"tractor.dev/wanix/web/runtime"
)

//go:embed v86.worker.min.js
var v86Bundle []byte

type VM struct {
	id         string
	kind       string
	serial     *serialReadWriter
	screenport js.Value
	shmpipe    *jsutil.PortReadWriter
}

func New(id, kind string) wanix.Resource {
	return &VM{
		id:   id,
		kind: kind,
	}
}

func (r *VM) ID() string {
	return r.id
}

// func (r *VM) Value() js.Value {
// 	return r.value
// }

func (r *VM) Open(name string) (fs.File, error) {
	return r.OpenContext(context.Background(), name)
}

func TryPatch(ctx context.Context, serial io.ReadWriter, serialFile *fskit.StreamFile) error {
	fsys, name, ok := fs.Origin(ctx)
	if !ok {
		return nil
	}
	ttyFile := path.Join(path.Dir(name), "ttyS0")
	if ok, err := fs.Exists(fsys, ttyFile); !ok {
		return fmt.Errorf("no ttyS0 file: %w", err)
	}
	tty, err := fsys.Open(ttyFile)
	if err != nil {
		return fmt.Errorf("open ttyS0 file: %w", err)
	}
	if fs.SameFile(tty, serialFile) {
		return nil
	}
	if w, ok := tty.(io.Writer); ok {
		go func() {
			_, err := io.Copy(serial, tty)
			if err != nil {
				log.Println("dom append-child: copy ttyS0 to serial:", err)
			}
		}()
		go func() {
			_, err := io.Copy(w, serial)
			if err != nil {
				log.Println("dom append-child: copy serial to ttyS0:", err)
			}
		}()
	}
	return nil
}

func (r *VM) OpenContext(ctx context.Context, name string) (fs.File, error) {
	serialFile := fskit.NewStreamFile(r.serial, r.serial, nil, fs.FileMode(0644))
	shmpipeFile := fskit.NewStreamFile(r.shmpipe, r.shmpipe, nil, fs.FileMode(0644))
	fsys := fskit.MapFS{
		"ctl": internal.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the resource",
			Run: func(_ *cli.Context, args []string) {
				switch args[0] {
				case "start":
					// todo: check if already started
					options, err := parseFlags(args[1:])
					if err != nil {
						log.Println("vm start:", err)
						return
					}
					serialport, screenport, shmpipe := makeVM(r.ID(), options, false)
					r.screenport = screenport
					r.shmpipe = shmpipe
					r.serial = newSerialReadWriter(serialport)
					if err := TryPatch(ctx, r.serial, serialFile); err != nil {
						log.Println("vm start:", err)
					}
					// fsys, _, ok := fs.Origin(ctx)
					// if ok {
					// 	cachedfs := metacache.New(fsys)
					// 	srv := p9.NewServer(p9kit.Attacher(cachedfs, p9kit.WithMemAttrStore()), p9.WithServerLogger(ulog.Log))
					// 	log.Println("starting 9p server for shmpipe")
					// 	go srv.Handle(shmpipe, shmpipe)
					// }
				case "update-screen":
					if r.screenport.IsUndefined() {
						log.Println("vm update-screen: screenport not found")
						return
					}
					channel := js.Global().Get("MessageChannel").New()
					js.Global().Get("top").Call("postMessage", map[string]any{
						"type":  "load",
						"token": args[1],
						"reply": channel.Get("port1"),
					}, "*", []any{channel.Get("port1")})
					channel.Get("port2").Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
						screen := args[0].Get("data").Get("transfers").Index(0)
						input := args[0].Get("data").Get("transfers").Index(1)
						r.screenport.Call("postMessage", map[string]any{
							"id":     "screen", // we dont even use id but to validate the message
							"screen": screen,
							"input":  input,
						}, []any{screen, input})
						return nil
					}))

				}
			},
		}),
	}
	if r.serial != nil {
		fsys["ttyS0"] = fskit.FileFS(serialFile, "ttyS0")
	}
	if r.shmpipe != nil {
		fsys["shmpipe0"] = fskit.FileFS(shmpipeFile, "shmpipe0")
	}
	return fs.OpenContext(ctx, fsys, name)
}

// careful, not running in worker will break text inputs on page
func makeVM(id string, options map[string]any, inWorker bool) (js.Value, js.Value, *jsutil.PortReadWriter) {
	var src []any
	var readyChannel js.Value
	if inWorker {
		src = []any{"var thisWorker = self; var process = undefined;", string(v86Bundle)}
	} else {
		readyChannel = js.Global().Get("MessageChannel").New()
		// todo: this is a bit of a hack, but it works
		js.Global().Set("vmReadyPort", readyChannel.Get("port2"))
		src = []any{"var thisWorker = window.vmReadyPort; var process = undefined;", string(v86Bundle)}
	}
	blob := js.Global().Get("Blob").New(js.ValueOf(src), js.ValueOf(map[string]any{"type": "text/javascript"}))
	url := js.Global().Get("URL").Call("createObjectURL", blob)

	var readyReceiver js.Value
	var readySender js.Value

	if inWorker {
		log.Println("starting worker")
		worker := js.Global().Get("Worker").New(url)
		worker.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
			js.Global().Get("console").Call("error", args[0])
			return nil
		}))
		worker.Set("onmessageerror", js.FuncOf(func(this js.Value, args []js.Value) any {
			js.Global().Get("console").Call("error", args[0])
			return nil
		}))
		readySender = worker
		readyReceiver = worker
	} else {
		log.Println("starting in-process")
		readySender = readyChannel.Get("port1")
		readyReceiver = readyChannel.Get("port1")
		jsutil.LoadScript(url.String(), false)
	}

	ready := make(chan js.Value)
	readyReceiver.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
		if !args[0].Get("data").Get("shmPort").IsUndefined() {
			ready <- args[0].Get("data").Get("shmPort")
		}
		if !args[0].Get("data").Get("audio").IsUndefined() {
			if !js.Global().Get("handleAudio").IsUndefined() {
				js.Global().Get("handleAudio").Invoke(args[0].Get("data"))
			}
		}
		return nil
	}))
	sys := runtime.Instance().Call("createPort")
	serialch := js.Global().Get("MessageChannel").New()
	p9ch := js.Global().Get("MessageChannel").New()
	p9ch.Get("port1").Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
		runtime.Instance().Call("_virtioHandle", args[0].Get("data"), js.FuncOf(func(this js.Value, args []js.Value) any {
			p9ch.Get("port1").Call("postMessage", args[0])
			return nil
		}))
		return nil
	}))

	data := map[string]any{
		"id":      id,
		"sys":     sys,
		"p9":      p9ch.Get("port2"),
		"serial":  serialch.Get("port2"),
		"options": options,
	}
	transfer := []any{sys, p9ch.Get("port2"), serialch.Get("port2")}
	if !runtime.Instance().Get("screen").IsUndefined() {
		data["screen"] = runtime.Instance().Get("screen")
		transfer = append(transfer, runtime.Instance().Get("screen"))
	}
	if !runtime.Instance().Get("input").IsUndefined() {
		data["input"] = runtime.Instance().Get("input").Get("port")
		transfer = append(transfer, runtime.Instance().Get("input").Get("port"))
	}
	readySender.Call("postMessage", data, transfer)
	bigpipe := jsutil.NewPortReadWriter(<-ready)

	return serialch.Get("port1"), readySender, bigpipe
}

// todo: replace with PortReadWriter
type serialReadWriter struct {
	js.Value
	buf *pipe.Buffer
}

func newSerialReadWriter(serialport js.Value) *serialReadWriter {
	buf := pipe.NewBuffer(true)
	serialport.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
		buf.Write([]byte{byte(args[0].Get("data").Int())})
		return nil
	}))
	return &serialReadWriter{
		Value: serialport,
		buf:   buf,
	}
}

func (s *serialReadWriter) Write(p []byte) (n int, err error) {
	buf := js.Global().Get("Uint8Array").New(len(p))
	n = js.CopyBytesToJS(buf, p)
	s.Value.Call("postMessage", buf)
	return
}

func (s *serialReadWriter) Read(p []byte) (int, error) {
	return s.buf.Read(p)
}
