package main

import (
	"embed"
	"fmt"
	"syscall/js"

	fsutil "tractor.dev/toolkit-go/engine/fs"
	fsutil2 "tractor.dev/wanix/internal/fsutil"
	"tractor.dev/wanix/internal/jsutil"
	"tractor.dev/wanix/internal/mountablefs"
	"tractor.dev/wanix/kernel/fs"
	"tractor.dev/wanix/kernel/jazz"
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
	blob := js.Global().Get("bootfs").Get("syscall.js").Get("blob")
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

	// setup jazz if enabled
	v, err := jsutil.WanixSyscall("host.getItem", "jazz:enabled")
	if err != nil {
		panic(err)
	}
	if !v.IsNull() {
		fmt.Println("Setting up Jazz...")
		origin := js.Global().Get("location").Get("origin").String()
		jazzMod := jsutil.Await(js.Global().Call("import", origin+"/sys/cmd/kernel/jazz/jazz.min.js"))
		ret := jsutil.Await(jazzMod.Get("initJazz").Invoke(js.Global()))
		if !ret.IsNull() {
			fmt.Println("Mounting JazzFS...")
			fsutil.MkdirAll(kernel.fs.FS(), "grp", 0755)
			err := kernel.fs.FS().(*mountablefs.FS).Mount(
				jazz.NewJazzFs(),
				"/grp",
			)
			if err != nil {
				fmt.Println(err)
			}
			ok, err := fsutil.Exists(kernel.fs.FS(), "grp/app/grp-todo")
			if err != nil {
				fmt.Println(err)
			}
			if !ok {
				fsutil.MkdirAll(kernel.fs.FS(), "grp/app", 0755)
				if err := fsutil2.CopyAll(kernel.fs.FS(), "sys/app/jazz-todo", "grp/app/grp-todo"); err != nil {
					fmt.Println(err)
				}
			}
		}

	}

	kernel.ui.InitializeJS()

	select {}
}

func version(this js.Value, args []js.Value) any {
	return Version
}
