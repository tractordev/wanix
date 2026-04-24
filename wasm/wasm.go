//go:build js && wasm

package main

import (
	"archive/tar"
	"io"
	"log"
	"path"
	"strconv"
	"syscall/js"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/web/sys"

	"tractor.dev/toolkit-go/duplex/mux"
	"tractor.dev/wanix"
	"tractor.dev/wanix/api"
	"tractor.dev/wanix/fs/memfs"
	"tractor.dev/wanix/fs/pipe"
	"tractor.dev/wanix/fs/signal"
	"tractor.dev/wanix/fs/tarfs"
	"tractor.dev/wanix/jsutil"
	"tractor.dev/wanix/term"
	"tractor.dev/wanix/vm"
	"tractor.dev/wanix/web"
)

func main() {
	log.SetFlags(log.Lshortfile)

	root, err := wanix.NewRoot()
	if err != nil {
		log.Fatal(err)
	}

	sysbindings := []struct {
		dst  string
		fsys fs.FS
	}{
		{"#term", term.New()},
		{"#web", web.New(root)},
		{"#vm", vm.New()},
		{"#pipe", &pipe.Allocator{}},
		{"#signal", &signal.Allocator{}},
		{"#ramfs", &memfs.Allocator{}},
	}
	for _, b := range sysbindings {
		if err := root.Namespace().Bind(b.fsys, ".", b.dst); err != nil {
			log.Fatal(err)
		}
	}

	el := sys.Element()
	el.Set("openPort", js.FuncOf(func(this js.Value, args []js.Value) any {
		ch := js.Global().Get("MessageChannel").New()
		port := el.Call("__portWrap", ch.Get("port1"))
		wr := &jsutil.Writer{Value: port}
		rd := &jsutil.Reader{Value: port}
		sess, err := mux.DialIO(wr, rd)
		if err != nil {
			log.Fatal(err)
		}

		go func(sess mux.Session, args []js.Value) {
			task := root
			if len(args) > 0 {
				t, err := root.Lookup(args[0].String())
				if err == nil {
					task = t
				}
			}
			api.Responder(sess, task)
		}(sess, args)

		return ch.Get("port2")
	}))

	bindings, err := jsutil.AwaitErr(el.Get("bindings"))
	if err != nil {
		log.Fatal(err)
	}
	for _, binding := range jsutil.ToSlice(bindings) {
		dst := binding.Get("dst").String()
		src := binding.Get("src").String()
		typ := binding.Get("type").String()
		switch typ {
		case "archive":
			go func() {
				v, err := jsutil.AwaitErr(binding.Get("archive"))
				if err != nil {
					log.Println("error fetching archive", err)
					return
				}
				archiveFS, err := tarfs.From(tar.NewReader(jsutil.NewReadableStream(v)))
				if err != nil {
					log.Println("error creating archive filesystem", err)
					return
				}
				if err := root.Namespace().Bind(archiveFS, ".", dst); err != nil {
					log.Println("error binding archive", err)
					return
				}
			}()
		case "fetch":
			go func() {
				resp, err := jsutil.AwaitErr(binding.Get("fetch"))
				if err != nil {
					log.Println("error fetching archive", err)
					return
				}
				reader := jsutil.NewReadableStream(resp.Get("body"))
				buf, err := io.ReadAll(reader)
				if err != nil {
					log.Println("error reading fetch", err)
					return
				}
				filefs := memfs.New()
				if err := fs.WriteFile(filefs, path.Base(dst), buf, 0644); err != nil {
					log.Println("error writing fetch", err)
					return
				}
				if err := root.Namespace().Bind(filefs, path.Base(dst), dst); err != nil {
					log.Println("error binding fetch", err)
					return
				}
			}()
		case "ns":
			if err := root.Bind(src, dst); err != nil {
				log.Fatal(err)
			}
		default:
		}
	}

	files, err := jsutil.AwaitErr(el.Get("files"))
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range jsutil.ToSlice(files) {
		dst := file.Get("dst").String()
		mode := file.Get("mode").String()
		encoding := file.Get("encoding").String()
		content := file.Get("content").String()
		// Convert mode string (like "644" or "0644") to int
		perm, err := strconv.ParseInt(mode, 8, 32)
		if err != nil {
			log.Fatalf("invalid file mode %q: %v", mode, err)
		}

		var data []byte
		switch encoding {
		case "", "utf-8", "utf8":
			data = []byte(content)
		default:
			log.Fatalf("unsupported encoding %q for file %s, skipping", encoding, dst)
		}

		if err := fs.WriteFile(root.Namespace(), dst, data, fs.FileMode(perm)); err != nil {
			log.Fatalf("error writing file %s: %v", dst, err)
		}
	}

	// bundleBytes := inst.Get("_bundle")
	// if !bundleBytes.IsUndefined() {
	// 	jsBuf := js.Global().Get("Uint8Array").New(bundleBytes)
	// 	b := make([]byte, jsBuf.Length())
	// 	js.CopyBytesToGo(b, jsBuf)
	// 	buf := bytes.NewBuffer(b)
	// 	bundleFS := tarfs.From(tar.NewReader(buf))

	// 	// ideally we could bind a memfs over bundleFS, but
	// 	// that still doesn't seem to be working yet
	// 	rw := memfs.New()
	// 	if err := fs.CopyFS(bundleFS, ".", rw, "."); err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	root.Namespace().Bind(rw, ".", "#bundle")
	// 	// root.Bind("#bundle", "bundle")
	// } else {
	// bundleURL := inst.Get("_bundleURL")
	// if !bundleURL.IsUndefined() {
	// 	bundle, err := jsutil.FetchToReader(bundleURL.String())
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	bundleFS := tarfs.From(tar.NewReader(bundle))
	// 	rw := memfs.New()
	// 	if err := fs.CopyFS(bundleFS, ".", rw, "."); err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	root.Namespace().Bind(rw, ".", "#bundle")
	// 	root.Bind("#bundle", "bundle")

	// 	// setup vm
	// 	vmraw, err := fs.ReadFile(root.Namespace(), "vm/new/default")
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	vm := strings.TrimSpace(string(vmraw))
	// 	if err := root.Bind("#console/data1", fmt.Sprintf("vm/%s/ttyS0", vm)); err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	cmdline := []string{
	// 		"init=/bin/init",
	// 		"rw",
	// 		"root=host9p",
	// 		"rootfstype=9p",
	// 		fmt.Sprintf("rootflags=trans=virtio,version=9p2000.L,aname=bundle/rootfs,cache=none,msize=131072", vm),
	// 		"loglevel=3",
	// 	}
	// 	ctlcmd := []string{
	// 		"start",
	// 		"-m", "1G",
	// 		"-append", fmt.Sprintf("'%s'", strings.Join(cmdline, " ")),
	// 	}
	// 	// boot vm as early as possible
	// 	log.Println("booting vm")
	// 	if err := fs.WriteFile(root.Namespace(), fmt.Sprintf("vm/%s/ctl", vm), []byte(strings.Join(ctlcmd, " ")), 0755); err != nil {
	// 		log.Fatal(err)
	// 	}
	// }
	// }

	// go virtio9p.Serve(root.Namespace(), inst, false)

	sys.Element().Call("__wasmReady")
	select {}
}
