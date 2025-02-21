//go:build js && wasm

package main

import (
	"log"
	"syscall/js"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/internal/virtio9p"
	"tractor.dev/wanix/kernel"
	"tractor.dev/wanix/web"
	"tractor.dev/wanix/web/api"
)

func main() {
	ctx := js.Global().Get("wanix")
	if ctx.IsUndefined() {
		log.Fatal("Wanix not initialized on this page")
	}

	k := kernel.New()
	k.AddModule("#web", web.New(k, ctx))

	root, err := k.NewRoot()
	if err != nil {
		log.Fatal(err)
	}

	root.Bind("#fsys", "fsys")
	root.Bind("#proc", "proc")
	root.Bind("#web", "web")

	// a, b := misc.BufferedConnPipe()
	// root.Namespace().Bind(fskit.OpenFunc(func(ctx context.Context, path string) (fs.File, error) {
	// 	return &proc.ConnFile{Conn: a, Name: "debug"}, nil
	// }), ".", "debug", "")
	// go func() {
	// 	for {
	// 		log.Println("writing")
	// 		b.Write([]byte("hello\n"))
	// 		time.Sleep(time.Second)
	// 	}
	// }()

	fs.ReadFile(root.Namespace(), "fsys/new/opfs")
	fs.WriteFile(root.Namespace(), "fsys/1/ctl", []byte("mount"), 0755)
	fs.WriteFile(root.Namespace(), "#proc/1/ctl", []byte("bind fsys/1/mount opfs"), 0755)

	virtio9p.StartFor(root.Namespace(), ctx, false)
	api.PortResponder(ctx.Get("sys"), root)
}
