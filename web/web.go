//go:build js && wasm

package web

import (
	"syscall/js"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/kernel"
	"tractor.dev/wanix/web/dom"
	"tractor.dev/wanix/web/fsa"
	"tractor.dev/wanix/web/sw"
	"tractor.dev/wanix/web/vm"
	"tractor.dev/wanix/web/worker"
)

func New(k *kernel.K, ctx js.Value) fskit.MapFS {
	k.Fsys.Register("pickerfs", func(s []string) (fs.FS, error) {
		return fsa.ShowDirectoryPicker(), nil
	})
	k.Fsys.Register("opfs", func(s []string) (fs.FS, error) {
		return fsa.OPFS()
	})
	webfs := fskit.MapFS{
		"dom":    dom.New(),
		"vm":     vm.New(),
		"worker": worker.New(k),
	}
	if !ctx.Get("sw").IsUndefined() {
		webfs["sw"] = sw.Activate(ctx.Get("sw"), k)
	}

	return webfs
}
