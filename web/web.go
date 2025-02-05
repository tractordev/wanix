//go:build js && wasm

package web

import (
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/kernel/fsys"
	"tractor.dev/wanix/web/dom"
	"tractor.dev/wanix/web/fsa"
	"tractor.dev/wanix/web/vm"
	"tractor.dev/wanix/web/worker"
)

func New(fsys *fsys.Device) fskit.MapFS {
	fsys.Register("pickerfs", func(s []string) (fs.FS, error) {
		return fsa.ShowDirectoryPicker(), nil
	})
	fsys.Register("opfs", func(s []string) (fs.FS, error) {
		return fsa.OPFS()
	})
	return fskit.MapFS{
		"dom":    dom.New(),
		"vm":     vm.New(),
		"worker": worker.New(),
	}
}
