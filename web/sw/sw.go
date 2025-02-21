//go:build js && wasm

package sw

import (
	"bytes"
	"context"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"syscall/js"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/internal/httprecorder"
	"tractor.dev/wanix/kernel"
	"tractor.dev/wanix/misc"
	"tractor.dev/wanix/web/jsutil"
)

type Device struct {
	active js.Value
	k      *kernel.K
}

func Activate(ch js.Value, k *kernel.K) *Device {
	reg := jsutil.Await(jsutil.Get("navigator.serviceWorker").Call("getRegistration"))
	if reg.IsUndefined() {
		swPath := "./sw.js"
		jsutil.Await(jsutil.Get("navigator.serviceWorker").Call("register", swPath, map[string]any{"type": "module"}))
		reg = jsutil.Await(jsutil.Get("navigator.serviceWorker.ready"))
	}

	d := &Device{
		active: reg.Get("active"),
		k:      k,
	}
	ch.Get("port2").Set("onmessage", js.FuncOf(d.handleMessage))

	reg.Get("active").Call("postMessage", map[string]any{"listen": ch.Get("port1")}, []any{ch.Get("port1")})
	return d
}

func (d *Device) Sub(name string) (fs.FS, error) {
	fsys := fskit.MapFS{
		"ctl": misc.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the active service worker",
			Run: func(ctx *cli.Context, args []string) {
				switch args[0] {
				case "start":

					// d.state = "running"
				case "terminate":
					// if !r.worker.IsUndefined() {
					// 	r.worker.Call("terminate")
					// }
					// d.state = "terminated"
				}
			},
		}),
		"state": misc.FieldFile(d.active.Get("state").String()),
		// "err": misc.FieldFile(r.state, nil),
		// "fsys": misc.FieldFile(r.fs, nil),
	}
	return fs.Sub(fsys, name)
}

func (d *Device) Open(name string) (fs.File, error) {
	return d.OpenContext(context.Background(), name)
}

func (d *Device) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, err := d.Sub(".")
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, name)
}

func (d *Device) handleMessage(this js.Value, args []js.Value) interface{} {
	if args[0].Get("data").Get("request").IsUndefined() {
		return nil
	}

	// Create a goroutine to handle the request asynchronously
	go func() {
		jsReq := args[0].Get("data").Get("request")
		jsResp := args[0].Get("data").Get("responder")

		req, err := http.NewRequest(jsReq.Get("method").String(), jsReq.Get("url").String(), nil)
		if err != nil {
			jsResp.Call("postMessage", js.ValueOf(map[string]interface{}{
				"status":     500,
				"statusText": "Gateway error",
				"body":       err.Error(),
			}))
			return
		}
		rw := httprecorder.NewRecorder()

		d.ServeHTTP(rw, req)

		headers := make(map[string]any)
		for k, v := range rw.Result().Header {
			headers[k] = v[0]
		}
		var buf bytes.Buffer
		rw.Body.WriteTo(&buf)

		jsBuf := js.Global().Get("Uint8Array").New(buf.Len())
		js.CopyBytesToJS(jsBuf, buf.Bytes())

		jsResp.Call("postMessage", js.ValueOf(map[string]interface{}{
			"status":     rw.Code,
			"statusText": rw.Result().Status[4:],
			"body":       jsBuf,
			"headers":    headers,
		}))
	}()

	return nil
}

func (d *Device) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/sw/") {
		path := strings.TrimPrefix(r.URL.Path, "/sw/")
		b, err := fs.ReadFile(d.k.NS, path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Add("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Add("Cross-Origin-Embedder-Policy", "require-corp")
		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = http.DetectContentType(b)
		}
		w.Header().Add("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		w.Write(b)
		return
	}
	w.WriteHeader(http.StatusContinue)
}
