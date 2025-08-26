//go:build js && wasm

package v86

import (
	"context"
	"fmt"
	"io"
	"log"
	"path"
	"syscall/js"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/internal"
	"tractor.dev/wanix/vfs/pipe"
)

type VM struct {
	id     string
	kind   string
	value  js.Value
	serial *serialReadWriter
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

func (r *VM) Value() js.Value {
	return r.value
}

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
					r.value = makeVM(r.ID(), options)
					r.serial = newSerialReadWriter(r.value)
					if err := TryPatch(ctx, r.serial, serialFile); err != nil {
						log.Println("vm start:", err)
					}
					r.value.Get("ready").Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
						r.value.Call("run")
						return nil
					}))
				}
			},
		}),
	}
	if r.serial != nil {
		fsys["ttyS0"] = fskit.FileFS(serialFile, "ttyS0")
	}
	return fs.OpenContext(ctx, fsys, name)
}

func makeVM(id string, options map[string]any) js.Value {
	vm := js.Global().Get("V86").New(options)
	readyPromise := js.Global().Get("Promise").New(js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		vm.Call("add_listener", "emulator-loaded", args[0])
		return nil
	}))
	vm.Set("ready", readyPromise)
	return vm
}

type serialReadWriter struct {
	js.Value
	buf *pipe.Buffer
}

func newSerialReadWriter(vm js.Value) *serialReadWriter {
	buf := pipe.NewBuffer(true)
	vm.Call("add_listener", "serial0-output-byte", js.FuncOf(func(this js.Value, args []js.Value) any {
		buf.Write([]byte{byte(args[0].Int())})
		return nil
	}))
	return &serialReadWriter{
		Value: vm,
		buf:   buf,
	}
}

func (s *serialReadWriter) Write(p []byte) (n int, err error) {
	buf := js.Global().Get("Uint8Array").New(len(p))
	n = js.CopyBytesToJS(buf, p)
	s.Value.Call("serial_send_bytes", 0, buf)
	return
}

func (s *serialReadWriter) Read(p []byte) (int, error) {
	return s.buf.Read(p)
}
