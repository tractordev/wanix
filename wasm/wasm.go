//go:build js && wasm

package main

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"path"
	"strconv"
	"syscall/js"

	"github.com/hugelgupf/p9/p9"
	"tractor.dev/toolkit-go/duplex/mux"
	"tractor.dev/wanix"
	"tractor.dev/wanix/api"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/memfs"
	"tractor.dev/wanix/fs/p9kit"
	"tractor.dev/wanix/fs/pipe"
	"tractor.dev/wanix/fs/signal"
	"tractor.dev/wanix/fs/tarfs"
	"tractor.dev/wanix/misc/jsutil"
	"tractor.dev/wanix/term"
	"tractor.dev/wanix/vm"
	"tractor.dev/wanix/web"
	"tractor.dev/wanix/web/jsfs"
	"tractor.dev/wanix/web/sys"
	"tractor.dev/wanix/web/worker"
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
		if err := root.NS().Bind(b.fsys, ".", b.dst); err != nil {
			log.Fatal(err)
		}
	}

	el := sys.Element()
	el.Set("_openPort", js.FuncOf(func(this js.Value, args []js.Value) any {
		ch := js.Global().Get("MessageChannel").New()
		port := el.Call("_portWrap", ch.Get("port1"))
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

	el.Set("_open9P", js.FuncOf(func(this js.Value, args []js.Value) any {
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
			srv := p9.NewServer(p9kit.Attacher(task.NS(), p9kit.WithMemAttrStore()), o...)
			if err := srv.Handle(ch.Reader(), ch.Writer()); err != nil {
				log.Fatal(err)
			}
		}()

		return ch.Port()
	}))

	el.Set("_updateTerminals", js.FuncOf(func(this js.Value, args []js.Value) any {
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

	el.Set("_setupNamespace", js.FuncOf(func(this js.Value, args []js.Value) any {
		taskID := args[0].String()
		baseFS := args[1].String()
		bindings := jsutil.ToSlice(args[2])
		task, err := root.Lookup(taskID)
		if err != nil {
			log.Fatal(err)
		}
		var resolve, reject js.Value
		promise := js.Global().Get("Promise").New(js.FuncOf(func(this js.Value, args []js.Value) any {
			resolve = args[0]
			reject = args[1]
			return js.Undefined()
		}))
		go func() {
			defer func() {
				if r := recover(); r != nil {
					reject.Invoke(js.ValueOf(r))
				}
			}()
			if taskID != "1" { // leave root namespace alone
				_ = baseFS
				// if err := task.NS().UnbindAll(); err != nil {
				// 	log.Println("error unbinding namespace", err)
				// 	return
				// }
				// if baseFS != "" && baseFS != "<null>" { // todo: better way to handle <null> upstream?
				// 	if err := task.NS().Bind(root.NS(), baseFS, "."); err != nil {
				// 		log.Println("error binding base filesystem", err, "baseFS", baseFS, "taskID", taskID)
				// 		return
				// 	}
				// }
			}
			for _, binding := range bindings {
				dst := binding.Get("dst").String()
				src := binding.Get("src").String()
				typ := binding.Get("type").String()
				// union := binding.Get("union").String()

				var mode fs.FileMode
				if !binding.Get("mode").IsUndefined() {
					// Convert mode string (like "644" or "0644") to int
					modeStr := binding.Get("mode").String()
					m, err := strconv.ParseInt(modeStr, 8, 32)
					if err != nil {
						log.Fatalf("invalid file mode %q: %v", modeStr, err)
					}
					mode = fs.FileMode(m)
				}

				switch typ {
				case "archive":
					v, err := jsutil.AwaitErr(binding.Get("data"))
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
					if err := task.NS().Bind(rwfs, ".", dst); err != nil {
						log.Println("error binding archive", err)
						return
					}
				case "fetch":
					v, err := jsutil.AwaitErr(binding.Get("data"))
					if err != nil {
						log.Println("error fetching", err)
						return
					}
					buf, err := io.ReadAll(jsutil.NewReadableStream(v))
					if err != nil {
						log.Println("error reading fetch", err)
						return
					}
					filefs := memfs.New()
					if err := fs.WriteFile(filefs, path.Base(dst), buf, mode); err != nil {
						log.Println("error writing fetch", err)
						return
					}
					if err := task.NS().Bind(filefs, path.Base(dst), dst); err != nil {
						log.Println("error binding fetch", err)
						return
					}
				case "file":
					v, err := jsutil.AwaitErr(binding.Get("data"))
					if err != nil {
						log.Println("error fetching", err)
						return
					}
					buf, err := io.ReadAll(jsutil.NewReadableStream(v))
					if err != nil {
						log.Println("error reading fetch", err)
						return
					}
					if err := fs.WriteFile(task.NS(), dst, buf, mode); err != nil {
						log.Fatalf("error writing file %s: %v", dst, err)
					}
				case "ns":
					// jsutil.Log("binding ns", src, dst, task.ID())
					if err := task.Bind(src, dst); err != nil {
						log.Fatal(err)
					}
				default:
					reject.Invoke(fmt.Errorf("unknown binding type %q", typ))
				}
			}
			resolve.Invoke(js.Undefined())
		}()
		return promise
	}))

	sys.Element().Call("_wasmReady")

	select {}
}
