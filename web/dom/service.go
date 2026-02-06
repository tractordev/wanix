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
	"tractor.dev/wanix/web/dom/xterm"
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
	fsys, rname, err := d.ResolveFS(ctx, name)
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, rname)
}

func (d *Service) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
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
					var termData *terminalReadWriter
					if name == "xterm" {
						xterm.Load()
						el = js.Global().Get("document").Call("createElement", "div")
						el.Set("className", "wanix-terminal")
						term := js.Global().Get("Terminal").New(map[string]any{
							"fontFamily": `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace`,
							// "fontSize":   "24",
							// "theme": map[string]any{
							// 	// Background colors (columns 40m-47m)
							// 	"background": "#3f3f3f", // Default background
							// 	"black":      "#4d4d4d", // 40m - Dark gray/black
							// 	"red":        "#6f5050", // 41m - Dark red
							// 	"green":      "#63b38b", // 42m - Green
							// 	"yellow":     "#efdeb1", // 43m - Teal/cyan
							// 	"blue":       "#51606f", // 44m - Dark blue/navy
							// 	"magenta":    "#db8ec2", // 45m - Purple/magenta
							// 	"cyan":       "#8ed0d2", // 46m - Light blue/cyan
							// 	"white":      "#dcdccd", // 47m - Light gray

							// 	// Foreground colors (regular and bright variants)
							// 	"foreground":    "#d9d9ca", // Default text color
							// 	"brightBlack":   "#4e4e4e", // Bright black/gray
							// 	"brightRed":     "#b69393", // Bright red
							// 	"brightGreen":   "#64b58d", // Bright green
							// 	"brightYellow":  "#ffeec1", // Bright yellow
							// 	"brightBlue":    "#91a2b3", // Bright blue
							// 	"brightMagenta": "#ffb3ea", // Bright magenta
							// 	"brightCyan":    "#aaeef1", // Bright cyan
							// 	"brightWhite":   "#eeedde", // Bright white

							// 	// Other properties
							// 	"cursor":                      "#ffffff",
							// 	"cursorAccent":                "#000000",
							// 	"selectionBackground":         "#4444ff",
							// 	"selectionForeground":         "#ffffff",
							// 	"selectionInactiveBackground": "#666666",
							// },
						})
						el.Set("term", term)
						fitAddon := js.Global().Get("FitAddon").Get("FitAddon").New()
						term.Call("loadAddon", fitAddon)
						term.Set("fitAddon", fitAddon)
						termData = newTerminalReadWriter(term)
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
	return fs.Resolve(fskit.UnionFS{fsys, fskit.MapFS(d.resources)}, ctx, name)
}
