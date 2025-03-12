//go:build js && wasm

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"log"
	"syscall/js"

	"tractor.dev/wanix"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/tarfs"
	"tractor.dev/wanix/web"
	"tractor.dev/wanix/web/api"
	"tractor.dev/wanix/web/virtio9p"
)

func main() {
	ctx := js.Global().Get("wanix")
	if ctx.IsUndefined() {
		log.Fatal("Wanix not initialized on this page")
	}

	// Download shell
	shellfs := make(chan *tarfs.FS, 1)
	promise := js.Global().Call("fetch", "/shell/shell.tgz")
	promise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		response := args[0]
		if !response.Get("ok").Bool() {
			log.Fatal("shell: Failed to download shell.tgz")
		}
		return response.Call("arrayBuffer")
	})).Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		go func() {
			jsBuf := js.Global().Get("Uint8Array").New(args[0])
			buf := make([]byte, jsBuf.Length())
			js.CopyBytesToGo(buf, jsBuf)
			gzReader, err := gzip.NewReader(bytes.NewReader(buf))
			if err != nil {
				log.Fatal("shell: Failed to create gzip reader:", err)
			}
			shellfs <- tarfs.Load(tar.NewReader(gzReader))
			gzReader.Close()
		}()
		return nil
	}))

	k := wanix.New()
	k.AddModule("#web", web.New(k, ctx))

	root, err := k.NewRoot()
	if err != nil {
		log.Fatal(err)
	}

	root.Bind("#task", "task")
	root.Bind("#cap", "cap")
	root.Bind("#web", "web")

	ro := <-shellfs
	rw := fskit.MemFS{}
	if err := fs.CopyFS(ro, ".", rw, "."); err != nil {
		log.Fatal("shell: failed to copy into memfs:", err)
	}
	root.Namespace().Bind(rw, ".", "#shell", "")

	virtio9p.StartFor(root.Namespace(), ctx, false)
	api.PortResponder(ctx.Get("sys"), root)
}
