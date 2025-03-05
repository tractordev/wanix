//go:build js && wasm

package dom

import (
	"context"
	"strconv"
	"strings"
	"syscall/js"

	"tractor.dev/wanix"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

type Service struct {
	types     map[string]func([]string) (fs.FS, error)
	resources map[string]fs.FS
	nextID    int
	k         *wanix.K
}

func New(k *wanix.K) *Service {
	d := &Service{
		types:     make(map[string]func([]string) (fs.FS, error)),
		resources: make(map[string]fs.FS),
		nextID:    0,
		k:         k,
	}
	for _, tag := range []string{"div", "iframe", "xterm", "script"} {
		d.Register(tag, func(args []string) (fs.FS, error) {
			return nil, nil
		})
	}
	return d
}

func (d *Service) Register(kind string, factory func([]string) (fs.FS, error)) {
	d.types[kind] = factory
}

func (d *Service) Open(name string) (fs.File, error) {
	return d.OpenContext(context.Background(), name)
}

func (d *Service) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, err := d.Sub(".")
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, name)
}

func (d *Service) Sub(name string) (fs.FS, error) {
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
	return fs.Sub(fskit.UnionFS{fsys, fskit.MapFS(d.resources)}, name)
}
