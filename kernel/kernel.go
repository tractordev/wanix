package kernel

import (
	"io/fs"

	"tractor.dev/wanix/kernel/fsys"
	"tractor.dev/wanix/kernel/ns"
	"tractor.dev/wanix/kernel/proc"
)

type K struct {
	Fsys *fsys.Device
	Proc *proc.Device
	Mod  map[string]fs.FS

	nsch chan *ns.FS
	NS   *ns.FS
	Root *proc.Process
}

func New() *K {
	nsch := make(chan *ns.FS, 1)
	return &K{
		Fsys: fsys.New(nsch),
		Proc: proc.New(),
		Mod:  make(map[string]fs.FS),
		nsch: nsch,
	}
}

func (k *K) AddModule(name string, mod fs.FS) {
	k.Mod[name] = mod
}

func (k *K) NewRoot() (*proc.Process, error) {
	p, err := k.Proc.Alloc("ns")
	if err != nil {
		return nil, err
	}

	// kludge: give the kernel a namespace / root proc
	// for modules that need it (web/sw, web/worker)
	k.Root = p
	k.NS = p.Namespace()
	k.nsch <- k.NS
	// bind hidden kernel devices
	if err := p.Namespace().Bind(k.Fsys, ".", "#fsys", ""); err != nil {
		return nil, err
	}
	if err := p.Namespace().Bind(k.Proc, ".", "#proc", ""); err != nil {
		return nil, err
	}

	for name, mod := range k.Mod {
		if err := p.Namespace().Bind(mod, ".", name, ""); err != nil {
			return nil, err
		}
	}

	return p, nil
}
