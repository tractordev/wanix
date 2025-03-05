//go:build js && wasm

package web

import (
	"fmt"
	"strings"
	"syscall/js"

	"tractor.dev/wanix"
	"tractor.dev/wanix/cap"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
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
