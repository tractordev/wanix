//go:build js && wasm

package kernel

import (
	"tractor.dev/wanix/kernel/fsys"
	"tractor.dev/wanix/kernel/proc"
	"tractor.dev/wanix/web"
)

type K struct {
	Fsys *fsys.Device
	Proc *proc.Device
	Web  *web.Module
}

func New() *K {
	fsys := fsys.New()
	k := &K{
		Fsys: fsys,
		Proc: proc.New(),
		Web:  web.New(fsys),
	}
	return k
}

func (k *K) NewRoot() (*proc.Process, error) {
	p, err := k.Proc.Alloc("ns")
	if err != nil {
		return nil, err
	}

	// bind hidden kernel devices
	if err := p.Namespace().Bind(k.Fsys, ".", "#fsys", ""); err != nil {
		return nil, err
	}
	if err := p.Namespace().Bind(k.Proc, ".", "#proc", ""); err != nil {
		return nil, err
	}
	if err := p.Namespace().Bind(k.Web, ".", "#web", ""); err != nil {
		return nil, err
	}

	return p, nil
}
