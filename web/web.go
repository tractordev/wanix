//go:build js && wasm

package web

import (
	"context"
	"fmt"
	"strings"
	"syscall/js"

	"tractor.dev/wanix"
	"tractor.dev/wanix/cap"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/misc"
	"tractor.dev/wanix/task"
	"tractor.dev/wanix/web/dom"
	"tractor.dev/wanix/web/fsa"
	"tractor.dev/wanix/web/sw"
	"tractor.dev/wanix/web/vm"
	"tractor.dev/wanix/web/worker"
)

func New(k *wanix.K, ctx js.Value) fskit.MapFS {
	workerfs := worker.New(k)
	opfs, _ := fsa.OPFS()
	webfs := fskit.MapFS{
		"dom":    dom.New(k),
		"vm":     vm.New(),
		"worker": workerfs,
		"opfs":   opfs,
	}
	if !ctx.Get("sw").IsUndefined() {
		webfs["sw"] = sw.Activate(ctx.Get("sw"), k)
	}

	k.Cap.Register("pickerfs", func(_ *cap.Resource) (cap.Mounter, error) {
		return func(_ []string) (fs.FS, error) {
			return fsa.ShowDirectoryPicker(), nil
		}, nil
	})

	k.Cap.Register("ws", func(r *cap.Resource) (cap.Mounter, error) {
		return func(args []string) (fs.FS, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("ws: no url provided")
			}
			ws := js.Global().Get("WebSocket").New(args[0])
			ws.Set("binaryType", "arraybuffer")
			df := &dataFile{
				Value: ws,
				buf:   misc.NewBufferedPipe(true),
			}
			ws.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
				go func() {
					jsBuf := js.Global().Get("Uint8Array").New(args[0].Get("data"))
					buf := make([]byte, jsBuf.Length())
					n := js.CopyBytesToGo(buf, jsBuf)
					df.buf.Write(buf[:n])
				}()
				return nil
			}))
			r.Extra["data"] = fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
				return df, nil
			})
			return nil, nil
		}, nil
	})

	k.Task.Register("wasi", func(p *task.Process) error {
		w, err := workerfs.Alloc()
		if err != nil {
			return err
		}
		args := append([]string{fmt.Sprintf("pid=%s", p.ID()), "/wasi/worker.js"}, strings.Split(p.Cmd(), " ")...)
		return w.Start(args...)
	})

	return webfs
}

type dataFile struct {
	js.Value
	buf *misc.BufferedPipe
}

func (s *dataFile) Stat() (fs.FileInfo, error) {
	return fskit.Entry("data", 0644), nil
}

func (s *dataFile) Write(p []byte) (n int, err error) {
	buf := js.Global().Get("Uint8Array").New(len(p))
	n = js.CopyBytesToJS(buf, p)
	s.Value.Call("send", buf)
	return
}

func (s *dataFile) Read(p []byte) (int, error) {
	return s.buf.Read(p)
}

func (s *dataFile) Close() error {
	return nil
}
