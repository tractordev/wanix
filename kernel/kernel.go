//go:build !js && !wasm

package kernel

import (
	"tractor.dev/wanix/kernel/fsys"
	"tractor.dev/wanix/kernel/proc"
)

type K struct {
	Fsys *fsys.Device
	Proc *proc.Device
}

func New() *K {
	k := &K{
		Fsys: fsys.New(),
		Proc: proc.New(),
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

	return p, nil
}
