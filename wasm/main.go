//go:build js && wasm

package main

import (
	"log"
	"syscall/js"

	"tractor.dev/wanix"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/internal/virtio9p"
	"tractor.dev/wanix/web"
	"tractor.dev/wanix/web/api"
)

func main() {
	ctx := js.Global().Get("wanix")
	if ctx.IsUndefined() {
		log.Fatal("Wanix not initialized on this page")
	}

	k := wanix.New()
	k.AddModule("#web", web.New(k, ctx))

	root, err := k.NewRoot()
	if err != nil {
		log.Fatal(err)
	}

	root.Bind("#task", "task")
	root.Bind("#cap", "cap")
	root.Bind("#web", "web")

	root.Namespace().Bind(fskit.MemFS{}, ".", "tmp", "")
	fs.WriteFile(root.Namespace(), "#task/1/ctl", []byte("bind web/opfs opfs"), 0755)

	virtio9p.StartFor(root.Namespace(), ctx, false)
	api.PortResponder(ctx.Get("sys"), root)
}
