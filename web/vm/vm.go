//go:build js && wasm

package vm

import (
	"context"
	"io"
	"log"
	"path"
	"syscall/js"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/internal"
)

type VM struct {
	id     int
	typ    string
	value  js.Value
	serial *serial
}

func (r *VM) Value() js.Value {
	return r.value
}

func (r *VM) Open(name string) (fs.File, error) {
	return r.OpenContext(context.Background(), name)
}

func (r *VM) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys := fskit.MapFS{
		"ctl": internal.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the resource",
			Run: func(_ *cli.Context, args []string) {
				switch args[0] {
				case "start":
					fsys, name, ok := fs.Origin(ctx)
					if ok {
						ttyFile := path.Join(path.Dir(name), "ttyS0")
						if ok, err := fs.Exists(fsys, ttyFile); ok {
							if tty, err := fsys.Open(ttyFile); err == nil {
								go io.Copy(r.serial, tty)
								if w, ok := tty.(io.Writer); ok {
									go io.Copy(w, r.serial)
								}
							} else {
								log.Println("vm start:", err)
							}
						} else {
							log.Println("vm start: no ttyS0 file", err)
						}
					}
					r.value.Get("ready").Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
						r.value.Call("run")
						return nil
					}))
				}
			},
		}),
	}
	return fs.OpenContext(ctx, fsys, name)
}

type serial struct {
	js.Value
	buf *internal.BufferedPipe
}

func newSerial(vm js.Value) *serial {
	buf := internal.NewBufferedPipe(true)
	vm.Call("add_listener", "serial0-output-byte", js.FuncOf(func(this js.Value, args []js.Value) any {
		buf.Write([]byte{byte(args[0].Int())})
		return nil
	}))
	return &serial{
		Value: vm,
		buf:   buf,
	}
}

func (s *serial) Write(p []byte) (n int, err error) {
	buf := js.Global().Get("Uint8Array").New(len(p))
	n = js.CopyBytesToJS(buf, p)
	s.Value.Call("serial_send_bytes", 0, buf)
	return
}

func (s *serial) Read(p []byte) (int, error) {
	return s.buf.Read(p)
}
