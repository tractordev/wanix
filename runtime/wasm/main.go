//go:build js && wasm

package main

import (
	"archive/tar"
	"bytes"
	"log"
	"syscall/js"

	"tractor.dev/wanix"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/tarfs"
	"tractor.dev/wanix/vfs/pipe"
	"tractor.dev/wanix/vfs/ramfs"
	"tractor.dev/wanix/vm"
	"tractor.dev/wanix/web"
	"tractor.dev/wanix/web/api"
	"tractor.dev/wanix/web/virtio9p"
)

func main() {
	log.SetFlags(log.Lshortfile)

	inst := js.Global().Get("wanix")
	if inst.IsUndefined() {
		log.Fatal("Wanix not initialized on this page")
	}

	k := wanix.New()
	k.AddModule("#web", web.New(k, inst))
	k.AddModule("#vm", vm.New())
	k.AddModule("#pipe", &pipe.Allocator{})
	k.AddModule("#|", &pipe.Allocator{}) // alias for #pipe
	k.AddModule("#ramfs", &ramfs.Allocator{})

	root, err := k.NewRoot()
	if err != nil {
		log.Fatal(err)
	}

	root.Bind("#task", "task")
	root.Bind("#cap", "cap")
	root.Bind("#web", "web")
	root.Bind("#vm", "vm")
	root.Bind("#|", "#console")

	bundleBytes := inst.Get("bundle")
	if !bundleBytes.IsUndefined() {
		jsBuf := js.Global().Get("Uint8Array").New(bundleBytes)
		b := make([]byte, jsBuf.Length())
		js.CopyBytesToGo(b, jsBuf)
		buf := bytes.NewBuffer(b)
		bundleFS := tarfs.Load(tar.NewReader(buf))

		// ideally we could bind a memfs over bundleFS, but
		// that still doesn't seem to be working yet
		rw := fskit.MemFS{}
		if err := fs.CopyFS(bundleFS, ".", rw, "."); err != nil {
			log.Fatal(err)
		}
		root.Namespace().Bind(rw, ".", "#bundle")
		// root.Bind("#bundle", "bundle")
	}

	js.Global().Get("wanix").Set("connect", js.FuncOf(func(this js.Value, args []js.Value) any {
		ch := js.Global().Get("MessageChannel").New()
		go api.PortResponder(js.Global().Get("wanix").Call("_toport", ch.Get("port1")), root)
		return ch.Get("port2")
	}))

	// todo: remove this, use connect
	go api.PortResponder(inst.Get("sys"), root)

	js.Global().Get("wanix").Call("ready")

	virtio9p.Serve(root.Namespace(), inst, false)

}
