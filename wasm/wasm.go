//go:build js && wasm

package main

import (
	"archive/tar"
	"io"
	"log"
	"path"
	"strconv"
	"syscall/js"

	"github.com/hugelgupf/p9/p9"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/web/jsfs"
	"tractor.dev/wanix/web/sys"
	"tractor.dev/wanix/web/worker"

	"tractor.dev/toolkit-go/duplex/mux"
	"tractor.dev/wanix"
	"tractor.dev/wanix/api"
	"tractor.dev/wanix/fs/memfs"
	"tractor.dev/wanix/fs/p9kit"
	"tractor.dev/wanix/fs/pipe"
	"tractor.dev/wanix/fs/signal"
	"tractor.dev/wanix/fs/tarfs"
	"tractor.dev/wanix/misc/jsutil"
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
		{"#term", term.New(root)},
		{"#web", web.New(root)},
		{"#vm", vm.New(root)},
		{"#pipe", &pipe.Allocator{}},
		{"#signal", &signal.Allocator{}},
		{"#ramfs", &memfs.Allocator{}},
		{"#js", jsfs.NewFS(js.Global())},
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

	el.Set("open9P", js.FuncOf(func(this js.Value, args []js.Value) any {
		ch := NewP9Channel()

		go func() {
			task := root
			if len(args) > 0 {
				t, err := root.Lookup(args[0].String())
				if err == nil {
					task = t
				}
			}
			var o []p9.ServerOpt
			// o = append(o, p9.WithServerLogger(ulog.Log))
			srv := p9.NewServer(p9kit.Attacher(task.Namespace(), p9kit.WithMemAttrStore()), o...)
			if err := srv.Handle(ch.Reader(), ch.Writer()); err != nil {
				log.Fatal(err)
			}
		}()

		return ch.Port()
	}))

	el.Set("__updateTerminals", js.FuncOf(func(this js.Value, args []js.Value) any {
		// termEl := args[0]
		for _, task := range root.Tasks() {
			w := worker.FromTask(task)
			if w.IsUndefined() {
				continue
			}
			w.Call("postMessage", map[string]any{
				"screen": "foobar",
			})
		}
		return js.Undefined()
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
				rwfs := memfs.New()
				// t := time.Now()
				if err := fs.CopyFS(archiveFS, ".", rwfs, "."); err != nil {
					log.Println("error copying archive to memory filesystem", err)
					return
				}
				// log.Println("copied archive to memory filesystem in", time.Since(t))
				if err := root.Namespace().Bind(rwfs, ".", dst); err != nil {
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

	sys.Element().Call("__wasmReady")

	select {}
}
