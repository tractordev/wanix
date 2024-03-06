package main

import (
	"embed"
	"syscall/js"

	"tractor.dev/wanix/internal/jsutil"
	"tractor.dev/wanix/kernel/fs"
	"tractor.dev/wanix/kernel/proc"
	"tractor.dev/wanix/kernel/tty"
	"tractor.dev/wanix/kernel/web"
)

//go:embed *
var Source embed.FS

var Version string

type Kernel struct {
	proc    proc.Service
	tty     tty.Service
	fs      fs.Service
	gateway web.Gateway
	ui      web.UI
}

func main() {
	kernel := Kernel{}

	// import syscall.js
	blob := js.Global().Get("initfs").Get("syscall.js").Get("blob")
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	jsutil.Await(js.Global().Call("import", url))

	// expose syscalls
	js.Global().Get("api").Set("kernel", map[string]any{
		"version": js.FuncOf(version),
	})

	// Initialize Go subsystems first, then their JS components
	kernel.proc.Initialize()
	kernel.tty.Initialize(&kernel.proc)
	kernel.fs.Initialize(Source, &kernel.proc)

	kernel.proc.InitializeJS()
	kernel.tty.InitializeJS()
	kernel.fs.InitializeJS()
	kernel.gateway.InitializeJS()
	kernel.ui.InitializeJS()

	select {}
}

func version(this js.Value, args []js.Value) any {
	return Version
}
