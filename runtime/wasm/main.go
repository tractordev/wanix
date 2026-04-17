//go:build js && wasm

package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"log"
	"strings"
	"syscall/js"

	"github.com/hugelgupf/p9/p9"
	"tractor.dev/toolkit-go/duplex/mux"
	"tractor.dev/wanix"
	"tractor.dev/wanix/api"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/memfs"
	"tractor.dev/wanix/fs/p9kit"
	"tractor.dev/wanix/fs/pipe"
	"tractor.dev/wanix/fs/tarfs"
	"tractor.dev/wanix/vm"
	"tractor.dev/wanix/vm/v86/virtio9p"
	"tractor.dev/wanix/web"
	"tractor.dev/wanix/web/dom/xterm"
	"tractor.dev/wanix/web/jsutil"
	"tractor.dev/wanix/web/runtime"
)

func main() {
	log.SetFlags(log.Lshortfile)

	xterm.Load()

	inst := runtime.Instance()

	k := wanix.New()
	k.AddModule("#web", web.New(k))
	k.AddModule("#vm", vm.New())
	k.AddModule("#pipe", &pipe.Allocator{})
	k.AddModule("#|", &pipe.Allocator{}) // alias for #pipe
	k.AddModule("#ramfs", &memfs.Allocator{})

	root, err := k.NewRoot()
	if err != nil {
		log.Fatal(err)
	}

	// todo: let config define, otherwise default to these
	root.Bind("#task", "task")
	root.Bind("#cap", "cap")
	root.Bind("#web", "web")
	root.Bind("#vm", "vm")
	root.Bind("#|", "#console")

	// if err := root.Namespace().Bind(idbfs.New("test"), ".", "idbfs"); err != nil {
	// 	log.Fatal(err)
	// }

	inst.Set("createPort", js.FuncOf(func(this js.Value, args []js.Value) any {
		ch := js.Global().Get("MessageChannel").New()
		port := inst.Call("_portConn", ch.Get("port1"))
		wr := &jsutil.Writer{Value: port}
		rd := &jsutil.Reader{Value: port}
		sess, err := mux.DialIO(wr, rd)
		if err != nil {
			log.Fatal(err)
		}
		go api.PortResponder(sess, root)
		return ch.Get("port2")
	}))

	bundleBytes := inst.Get("_bundle")
	if !bundleBytes.IsUndefined() {
		jsBuf := js.Global().Get("Uint8Array").New(bundleBytes)
		b := make([]byte, jsBuf.Length())
		js.CopyBytesToGo(b, jsBuf)
		buf := bytes.NewBuffer(b)
		bundleFS := tarfs.From(tar.NewReader(buf))

		// ideally we could bind a memfs over bundleFS, but
		// that still doesn't seem to be working yet
		rw := memfs.New()
		if err := fs.CopyFS(bundleFS, ".", rw, "."); err != nil {
			log.Fatal(err)
		}
		root.Namespace().Bind(rw, ".", "#bundle")
		// root.Bind("#bundle", "bundle")
	} else {
		bundleURL := inst.Get("_bundleURL")
		if !bundleURL.IsUndefined() {
			bundle, err := jsutil.FetchToReader(bundleURL.String())
			if err != nil {
				log.Fatal(err)
			}
			bundleFS := tarfs.From(tar.NewReader(bundle))
			rw := memfs.New()
			if err := fs.CopyFS(bundleFS, ".", rw, "."); err != nil {
				log.Fatal(err)
			}
			root.Namespace().Bind(rw, ".", "#bundle")
			root.Bind("#bundle", "bundle")

			// setup vm
			vmraw, err := fs.ReadFile(root.Namespace(), "vm/new/default")
			if err != nil {
				log.Fatal(err)
			}
			vm := strings.TrimSpace(string(vmraw))
			if err := root.Bind("#console/data1", fmt.Sprintf("vm/%s/ttyS0", vm)); err != nil {
				log.Fatal(err)
			}
			cmdline := []string{
				"init=/bin/init",
				"rw",
				"root=host9p",
				"rootfstype=9p",
				fmt.Sprintf("rootflags=trans=virtio,version=9p2000.L,aname=bundle/rootfs,cache=none,msize=131072", vm),
				"loglevel=3",
			}
			ctlcmd := []string{
				"start",
				"-m", "1G",
				"-append", fmt.Sprintf("'%s'", strings.Join(cmdline, " ")),
			}
			// boot vm as early as possible
			log.Println("booting vm")
			if err := fs.WriteFile(root.Namespace(), fmt.Sprintf("vm/%s/ctl", vm), []byte(strings.Join(ctlcmd, " ")), 0755); err != nil {
				log.Fatal(err)
			}
		}
	}

	// r2fs := httpfs.New("https://r2fs.proteco.workers.dev/", nil)
	// opfs, err := fsa.OPFS("r2fs")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// sfs := syncfs.New(opfs, r2fs, 3*time.Second)
	// go func() {
	// 	if err := sfs.Sync(); err != nil {
	// 		log.Printf("err syncing: %v\n", err)
	// 	}
	// }()
	// if err := root.Namespace().Bind(sfs, ".", "#data"); err != nil {
	// 	log.Fatal(err)
	// }

	port := inst.Call("_portConn", inst.Get("_sys").Get("port1"))
	wr := &jsutil.Writer{Value: port}
	rd := &jsutil.Reader{Value: port}
	sess, err := mux.DialIO(wr, rd)
	if err != nil {
		log.Fatal(err)
	}
	go api.PortResponder(sess, root)

	// this is still a bit of a hack and wip
	export9p := inst.Get("config").Get("export9p")
	if export9p.IsUndefined() {
		export9p = js.ValueOf(false)
	}
	if export9p.Bool() {
		log.Println("exporting 9p")
		ws := js.Global().Get("WanixSocket").New("ws://localhost:7654/.well-known/export9p")
		sess := mux.New(&jsutil.ReadWriter{
			ReadCloser:  &jsutil.Reader{Value: ws},
			WriteCloser: &jsutil.Writer{Value: ws},
		})
		go func() {
			for {
				ch, err := sess.Accept()
				if err != nil {
					log.Println("9p export accept:", err)
					break
				}
				go func() {
					defer ch.Close()
					srv := p9.NewServer(p9kit.Attacher(root.Namespace(), p9kit.WithMemAttrStore()))
					if err := srv.Handle(ch, ch); err != nil {
						log.Println("9p export:", err)
						return
					}
				}()
			}
		}()

	}

	inst.Call("_wasmReady")

	debug := inst.Get("config").Get("debug9p")
	if debug.IsUndefined() {
		debug = js.ValueOf(false)
	}

	virtio9p.Serve(root.Namespace(), inst, debug.Bool())

}
