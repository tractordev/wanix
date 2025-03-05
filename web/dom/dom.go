//go:build js && wasm

package dom

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"syscall/js"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/kernel"
	"tractor.dev/wanix/misc"
	"tractor.dev/wanix/web/jsutil"
)

type Device struct {
	types     map[string]func([]string) (fs.FS, error)
	resources map[string]fs.FS
	nextID    int
	k         *kernel.K
}

func New(k *kernel.K) *Device {
	d := &Device{
		types:     make(map[string]func([]string) (fs.FS, error)),
		resources: make(map[string]fs.FS),
		nextID:    0,
		k:         k,
	}
	for _, tag := range []string{"div", "iframe", "xterm"} {
		d.Register(tag, func(args []string) (fs.FS, error) {
			return nil, nil
		})
	}
	return d
}

func (d *Device) Register(kind string, factory func([]string) (fs.FS, error)) {
	d.types[kind] = factory
}

func (d *Device) Open(name string) (fs.File, error) {
	return d.OpenContext(context.Background(), name)
}

func (d *Device) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys := fskit.MapFS{
		"new": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			if name == "." {
				var nodes []fs.DirEntry
				for kind := range d.types {
					nodes = append(nodes, fskit.Entry(kind, 0555))
				}
				return fskit.DirFile(fskit.Entry("new", 0555), nodes...), nil
			}
			return &fskit.FuncFile{
				Node: fskit.Entry(name, 0555),
				ReadFunc: func(n *fskit.Node) error {
					factory, ok := d.types[name]
					if !ok {
						return fs.ErrNotExist
					}
					d.nextID++
					rid := strconv.Itoa(d.nextID)
					var el js.Value
					var termData *termDataFile
					if name == "xterm" {
						el = js.Global().Get("document").Call("createElement", "div")
						term := js.Global().Get("Terminal").New()
						el.Set("term", term)
						termData = newTermData(term)
						setupFileDrop(el, d.k.NS)
					} else {
						el = js.Global().Get("document").Call("createElement", name)
					}
					d.resources[rid] = &Element{
						id:      d.nextID,
						typ:     name,
						factory: factory,
						value:   el,
						dom:     d,

						termData: termData,
					}
					fskit.SetData(n, []byte(rid+"\n"))
					return nil
				},
			}, nil
		}),
		"body": &Element{
			typ:   "body",
			value: js.Global().Get("document").Get("body"),
			dom:   d,
		},
		"style": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			return &fskit.FuncFile{
				Node: fskit.Entry("style", 0644),
				CloseFunc: func(n *fskit.Node) error {
					if len(n.Data()) == 0 {
						return nil
					}
					el := js.Global().Get("document").Call("createElement", "style")
					el.Set("innerText", strings.TrimSpace(string(n.Data())))
					js.Global().Get("document").Get("body").Call("appendChild", el)
					return nil
				},
			}, nil
		}),
	}
	return fs.OpenContext(ctx, fskit.UnionFS{fsys, fskit.MapFS(d.resources)}, name)
}

type Element struct {
	factory func([]string) (fs.FS, error)
	id      int
	typ     string
	value   js.Value
	dom     *Device

	termData *termDataFile
}

func (r *Element) Value() js.Value {
	return r.value
}

func (r *Element) Open(name string) (fs.File, error) {
	return r.OpenContext(context.Background(), name)
}

func (r *Element) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys := fskit.MapFS{
		"ctl": misc.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the resource",
			Run: func(ctx *cli.Context, args []string) {
				switch args[0] {
				case "append-child": // append-child <dom id>
					res, found := r.dom.resources[args[1]]
					if !found {
						return
					}
					el, ok := res.(*Element)
					if !ok {
						return
					}
					r.value.Call("appendChild", el.Value())
					if el.typ == "xterm" {
						el.Value().Get("term").Call("open", el.Value())
					}
				}
			},
		}),
		"type": misc.FieldFile(r.typ),
		"attrs": misc.FieldFile(
			// getter
			func() (string, error) {
				var builder strings.Builder
				names := r.value.Call("getAttributeNames")
				if names.Get("length").Int() == 0 {
					return "", nil
				}
				for i := 0; i < names.Get("length").Int(); i++ {
					name := names.Index(i).String()
					value := r.value.Call("getAttribute", name)
					fmt.Fprintf(&builder, "%s='%s'\n", name, value)
				}
				return builder.String(), nil
			},
			// setter
			func(data []byte) error {
				s := string(data)
				lines := strings.Split(s, "\n")
				for _, line := range lines {
					kv := strings.SplitN(line, "=", 2)
					if len(kv) != 2 {
						continue
					}
					r.value.Call("setAttribute", kv[0], strings.Trim(kv[1], "'"))
				}
				return nil
			},
		),
		"html": misc.FieldFile(func() (string, error) {
			return r.value.Get("outerHTML").String(), nil
		}),
		"text": misc.FieldFile(func() (string, error) {
			return r.value.Get("innerText").String(), nil
		}),
	}
	if r.typ == "xterm" {
		fsys["data"] = fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			return r.termData, nil
		})
	}
	return fs.OpenContext(ctx, fsys, name)
}

type termDataFile struct {
	js.Value
	buf *misc.BufferedPipe
}

func newTermData(term js.Value) *termDataFile {
	buf := misc.NewBufferedPipe(true)
	enc := js.Global().Get("TextEncoder").New()
	term.Call("onData", js.FuncOf(func(this js.Value, args []js.Value) any {
		jsbuf := enc.Call("encode", args[0])
		gobuf := make([]byte, jsbuf.Get("length").Int())
		js.CopyBytesToGo(gobuf, jsbuf)
		buf.Write(gobuf)
		return nil
	}))
	return &termDataFile{
		Value: term,
		buf:   buf,
	}
}

func (s *termDataFile) Stat() (fs.FileInfo, error) {
	return fskit.Entry("data", 0644), nil
}

func (s *termDataFile) Write(p []byte) (n int, err error) {
	buf := js.Global().Get("Uint8Array").New(len(p))
	n = js.CopyBytesToJS(buf, p)
	s.Value.Call("write", buf)
	return
}

func (s *termDataFile) Read(p []byte) (int, error) {
	return s.buf.Read(p)
}

func (s *termDataFile) Close() error {
	return nil
}

// TODO: handle multiple files, put in dir under opfs
func setupFileDrop(el js.Value, fsys fs.FS) {
	defaultHandler := js.FuncOf(func(this js.Value, args []js.Value) any {
		args[0].Call("preventDefault")
		args[0].Call("stopPropagation")
		return nil
	})
	el.Call("addEventListener", "dragenter", defaultHandler)
	el.Call("addEventListener", "dragover", defaultHandler)
	el.Call("addEventListener", "drop", js.FuncOf(func(this js.Value, args []js.Value) any {
		args[0].Call("preventDefault")
		args[0].Call("stopPropagation")
		file := args[0].Get("dataTransfer").Get("files").Index(0)
		if file.IsUndefined() {
			return nil
		}
		//term := args[0].Get("target").Call("closest", ".terminal").Get("parentElement").Get("term")
		go func() {
			jsBuf := js.Global().Get("Uint8Array").New(jsutil.Await(file.Call("arrayBuffer")))
			buf := make([]byte, jsBuf.Get("length").Int())
			js.CopyBytesToGo(buf, jsBuf)
			filename := "opfs/" + file.Get("name").String()
			if err := fs.WriteFile(fsys, filename, buf, 0644); err != nil {
				js.Global().Get("console").Call("error", err.Error())
				return
			}
		}()
		return nil
	}))
}
