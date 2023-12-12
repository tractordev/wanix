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

var version string

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

func (k *Kernel) Run(ctx context.Context) error {
	// import syscall.js
	blob := js.Global().Get("initfs").Get("syscall.js")
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	jsutil.Await(js.Global().Call("import", url))

	// expose syscalls
	js.Global().Get("api").Set("kernel", map[string]any{
		"version": js.FuncOf(k.version),
	})

	// initialize components
	for _, c := range k.Components {
		c.InitializeJS()
	}

	select {}
}

func (k *Kernel) Version() string {
	return version
}

func (k *Kernel) version(this js.Value, args []js.Value) any {
	return version
}
