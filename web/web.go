//go:build js && wasm

package web

import (
	"fmt"
	"strings"
	"syscall/js"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/kernel"
	"tractor.dev/wanix/kernel/proc"
	"tractor.dev/wanix/web/dom"
	"tractor.dev/wanix/web/fsa"
	"tractor.dev/wanix/web/sw"
	"tractor.dev/wanix/web/vm"
	"tractor.dev/wanix/web/worker"
)

func New(k *kernel.K, ctx js.Value) fskit.MapFS {
	workerfs := worker.New(k)
	webfs := fskit.MapFS{
		"dom":    dom.New(k),
		"vm":     vm.New(),
		"worker": workerfs,
	}
	if !ctx.Get("sw").IsUndefined() {
		webfs["sw"] = sw.Activate(ctx.Get("sw"), k)
	}

	k.Fsys.Register("pickerfs", func(s []string) (fs.FS, error) {
		return fsa.ShowDirectoryPicker(), nil
	})
	k.Fsys.Register("opfs", func(s []string) (fs.FS, error) {
		return fsa.OPFS()
	})

	k.Proc.Register("wasi", func(p *proc.Process) error {
		w, err := workerfs.Alloc()
		if err != nil {
			return err
		}
		args := append([]string{fmt.Sprintf("pid=%s", p.ID()), "/wasi/worker.js"}, strings.Split(p.Cmd(), " ")...)
		return w.Start(args...)
	})

	return webfs
}
