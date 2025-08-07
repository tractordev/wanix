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
	"tractor.dev/wanix/internal"
	"tractor.dev/wanix/web"
	"tractor.dev/wanix/web/api"
	"tractor.dev/wanix/web/jsutil"
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

	root, err := k.NewRoot()
	if err != nil {
		log.Fatal(err)
	}

	root.Bind("#task", "task")
	root.Bind("#cap", "cap")
	root.Bind("#web", "web")

	bundleBytes := inst.Get("bundle")
	if !bundleBytes.IsUndefined() {
		jsBuf := js.Global().Get("Uint8Array").New(bundleBytes)
		b := make([]byte, jsBuf.Length())
		js.CopyBytesToGo(b, jsBuf)
		buf := bytes.NewBuffer(b)
		bundleFS := tarfs.Load(tar.NewReader(buf))
		root.Namespace().Bind(bundleFS, ".", "#bundle")
		root.Bind("#bundle", "bundle")
	}

	shellfs, err := fetchTarballFS("/shell/shell.tgz")
	if err != nil {
		log.Fatal(err)
	}
	rw := fskit.MemFS{}
	// ideally we could bind memfs over shellfs, but
	// that still doesn't seem to be working yet
	if err := fs.CopyFS(shellfs, ".", rw, "."); err != nil {
		log.Fatal(err)
	}
	root.Namespace().Bind(rw, ".", "#shell")
	// root.Namespace().Bind(fskit.MemFS{}, ".", "#shell", "")

	// afs, err := fetchTarballFS("/shell/alpine.tgz")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// arw := fskit.MemFS{}
	// if err := fs.CopyFS(afs, ".", arw, "."); err != nil {
	// 	log.Fatal(err)
	// }
	// root.Namespace().Bind(arw, ".", "#alpine", "")

	go virtio9p.Serve(root.Namespace(), inst, false)
	api.PortResponder(inst.Get("sys"), root)
}

func fetchTarballFS(name string) (fs.FS, error) {
	u, err := internal.ParseURL(name)
	if err != nil {
		return nil, err
	}
	reader, err := jsutil.FetchToReader(u.String())
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	gzreader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	return tarfs.Load(tar.NewReader(gzreader)), nil
}
