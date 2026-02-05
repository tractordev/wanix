//go:build js && wasm

package web

import (
	"context"
	"fmt"
	"log"
	"strings"
	"syscall/js"

	"tractor.dev/wanix"
	"tractor.dev/wanix/cap"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	gojsworker "tractor.dev/wanix/runtime/gojs/worker"
	wasiworker "tractor.dev/wanix/runtime/wasi/worker"
	"tractor.dev/wanix/task"
	"tractor.dev/wanix/vfs/pipe"
	"tractor.dev/wanix/vm"
	"tractor.dev/wanix/web/caches"
	"tractor.dev/wanix/web/dl"
	"tractor.dev/wanix/web/dom"
	"tractor.dev/wanix/web/fsa"
	"tractor.dev/wanix/web/runtime"
	"tractor.dev/wanix/web/sw"
	"tractor.dev/wanix/web/worker"
)

func New(k *wanix.K) fskit.MapFS {
	workerfs := worker.New(k.Root)
	opfs, _ := fsa.OPFS()
	webfs := fskit.MapFS{
		"dom":    dom.New(k),
		"vm":     vm.New(),
		"caches": caches.New(),
		"worker": workerfs,
		"opfs":   opfs,
		"dl":     dl.New(),
	}
	if !runtime.Instance().Get("_sw").IsUndefined() {
		webfs["sw"] = sw.Activate(runtime.Instance().Get("_sw"), k)
		webfs["sw"] = sw.Activate(runtime.Instance().Get("_sw"), k)
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
				buf:   pipe.NewBuffer(true),
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

	k.Task.Register("wasi", func(p *task.Resource) error {
		w, err := workerfs.Alloc(p)
		if err != nil {
			return err
		}
		url := wasiworker.BlobURL()
		args := append([]string{fmt.Sprintf("pid=%s", p.ID()), url}, strings.Split(p.Cmd(), " ")...)
		return w.Start(args...)
	})

	k.Task.Register("gojs", func(p *task.Resource) error {
		w, err := workerfs.Alloc(p)
		if err != nil {
			return err
		}
		url := gojsworker.BlobURL()
		args := append([]string{fmt.Sprintf("pid=%s", p.ID()), url}, strings.Split(p.Cmd(), " ")...)
		log.Println("gojs task started")
		return w.Start(args...)
	})
	return webfs
}

type dataFile struct {
	js.Value
	buf *pipe.Buffer
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
