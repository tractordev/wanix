package wanix

import (
	"io/fs"

	"tractor.dev/wanix/cap"
	"tractor.dev/wanix/task"
	"tractor.dev/wanix/vfs"
)

type K struct {
	Cap  *cap.Service
	Task *task.Service
	Mod  map[string]fs.FS

	nsch chan *vfs.NS
	NS   *vfs.NS
	Root *task.Process
}

func New() *K {
	nsch := make(chan *vfs.NS, 1)
	return &K{
		Cap:  cap.New(nsch),
		Task: task.New(),
		Mod:  make(map[string]fs.FS),
		nsch: nsch,
	}
}

func (k *K) AddModule(name string, mod fs.FS) {
	k.Mod[name] = mod
}

func (k *K) NewRoot() (*task.Process, error) {
	p, err := k.Task.Alloc("ns")
	if err != nil {
		return nil, err
	}

	// kludge: give the kernel a namespace / root proc
	// for modules that need it (web/sw, web/worker)
	k.Root = p
	k.NS = p.Namespace()
	k.nsch <- k.NS
	// bind hidden kernel devices
	if err := p.Namespace().Bind(k.Cap, ".", "#cap", ""); err != nil {
		return nil, err
	}
	if err := p.Namespace().Bind(k.Task, ".", "#task", ""); err != nil {
		return nil, err
	}

	for name, mod := range k.Mod {
		if err := p.Namespace().Bind(mod, ".", name, ""); err != nil {
			return nil, err
		}
	}

	return p, nil
}
