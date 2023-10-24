package main

import (
	"context"
	"syscall/js"

	"tractor.dev/toolkit-go/engine"
	"tractor.dev/wanix/internal/jsutil"
	"tractor.dev/wanix/kernel/fs"
	"tractor.dev/wanix/kernel/proc"
	"tractor.dev/wanix/kernel/tty"
	"tractor.dev/wanix/kernel/web"
)

func main() {
	engine.Run(Kernel{},
		proc.Service{},
		tty.Service{},
		web.UI{},
		web.Gateway{},
		fs.Service{},
	)
}

type Component interface {
	InitializeJS()
}

type Kernel struct {
	Components []Component
}

func (m *Kernel) Run(ctx context.Context) error {
	// import syscall.js
	blob := js.Global().Get("initfs").Get("syscall.js")
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	jsutil.Await(js.Global().Call("import", url))

	// initialize components
	for _, c := range m.Components {
		c.InitializeJS()
	}

	select {}
}
