//go:build js && wasm

package web

import (
	"context"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/kernel/fsys"
	"tractor.dev/wanix/web/dom"
	"tractor.dev/wanix/web/fsa"
	"tractor.dev/wanix/web/vm"
	"tractor.dev/wanix/web/worker"
)

type Module struct {
	Dom    *dom.Device
	VM     *vm.Device
	Worker *worker.Device
}

func New(fsys *fsys.Device) *Module {
	w := &Module{
		Dom:    dom.New(),
		VM:     vm.New(),
		Worker: worker.New(),
	}
	fsys.Register("pickerfs", func(s []string) (fs.FS, error) {
		return fsa.ShowDirectoryPicker(), nil
	})
	fsys.Register("opfs", func(s []string) (fs.FS, error) {
		return fsa.OPFS()
	})
	return w
}

func (m *Module) Sub(name string) (fs.FS, error) {
	return fs.Sub(fskit.MapFS{
		"dom":    m.Dom,
		"vm":     m.VM,
		"worker": m.Worker,
	}, name)
}

func (m *Module) Open(name string) (fs.File, error) {
	return m.OpenContext(context.Background(), name)
}

func (m *Module) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, err := m.Sub(".")
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, name)
}
