//go:build js && wasm

package dom

import (
	"context"
	"fmt"
	"io"
	"log"
	"path"
	"strconv"
	"strings"
	"syscall/js"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/internal"
	"tractor.dev/wanix/vfs/pipe"
	"tractor.dev/wanix/web/jsutil"
)

type Element struct {
	factory func([]string) (fs.FS, error)
	id      int
	typ     string
	value   js.Value
	dom     *Service

	termData *terminalReadWriter
}

func (r *Element) ID() string {
	return strconv.Itoa(r.id)
}

func (r *Element) Value() js.Value {
	return r.value
}

func (r *Element) Open(name string) (fs.File, error) {
	return r.OpenContext(context.Background(), name)
}

func TryPatch(ctx context.Context, el *Element, termFile *fskit.StreamFile) error {
	fsys, name, ok := fs.Origin(ctx)
	if !ok {
		return nil
	}
	dataFile := path.Join(path.Dir(path.Dir(name)), el.ID(), "data")
	if ok, err := fs.Exists(fsys, dataFile); !ok {
		return fmt.Errorf("no data file: %s %w", dataFile, err)
	}
	data, err := fsys.Open(dataFile)
	if err != nil {
		return fmt.Errorf("open data file: %w", err)
	}
	if fs.SameFile(data, termFile) {
		return nil
	}
	if w, ok := data.(io.Writer); ok {
		go func() {
			_, err := io.Copy(el.termData, data)
			if err != nil {
				log.Println("dom append-child: copy data to term:", err)
			}
		}()
		go func() {
			_, err := io.Copy(w, el.termData)
			if err != nil {
				log.Println("dom append-child: copy term to data:", err)
			}
		}()
	}
	return nil
}

func (r *Element) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys := fskit.MapFS{
		"ctl": internal.ControlFile(&cli.Command{
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
						el.Value().Get("term").Get("fitAddon").Call("fit")
						termFile := fskit.NewStreamFile(el.termData, el.termData, nil, fs.FileMode(0644))
						if err := TryPatch(ctx, el, termFile); err != nil {
							log.Println("dom append-child:", err)
						}
					}
				case "remove": // remove
					delete(r.dom.resources, r.ID())
					r.value.Call("remove")
				case "reset": // reset
					// kludge: specific to xterm
					if r.typ == "xterm" {
						r.value.Get("term").Call("reset")
					}
				}
			},
		}),
		"type": internal.FieldFile(r.typ),
		"attrs": internal.FieldFile(
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
		"html": internal.FieldFile(func() (string, error) {
			return r.value.Get("outerHTML").String(), nil
		}),
		"text": internal.FieldFile(func() (string, error) {
			return r.value.Get("innerText").String(), nil
		}),
	}
	if r.typ == "xterm" && r.termData != nil {
		termFile := fskit.NewStreamFile(r.termData, r.termData, nil, fs.FileMode(0644))
		fsys["data"] = fskit.FileFS(termFile, "data")
	}
	return fs.OpenContext(ctx, fsys, name)
}

type terminalReadWriter struct {
	js.Value
	buf *pipe.Buffer
}

func newTerminalReadWriter(term js.Value) *terminalReadWriter {
	buf := pipe.NewBuffer(true)
	enc := js.Global().Get("TextEncoder").New()
	term.Call("onData", js.FuncOf(func(this js.Value, args []js.Value) any {
		jsbuf := enc.Call("encode", args[0])
		gobuf := make([]byte, jsbuf.Get("length").Int())
		js.CopyBytesToGo(gobuf, jsbuf)
		buf.Write(gobuf)
		return nil
	}))
	return &terminalReadWriter{
		Value: term,
		buf:   buf,
	}
}

func (s *terminalReadWriter) Write(p []byte) (n int, err error) {
	buf := js.Global().Get("Uint8Array").New(len(p))
	n = js.CopyBytesToJS(buf, p)
	s.Value.Call("write", buf)
	return
}

func (s *terminalReadWriter) Read(p []byte) (int, error) {
	return s.buf.Read(p)
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
			filename := "web/opfs/" + file.Get("name").String()
			if err := fs.WriteFile(fsys, filename, buf, 0644); err != nil {
				js.Global().Get("console").Call("error", err.Error())
				return
			}
		}()
		return nil
	}))
}
