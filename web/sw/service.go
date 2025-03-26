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
	"time"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/internal"
	"tractor.dev/wanix/internal/httprecorder"
	"tractor.dev/wanix/web/jsutil"
)

type Service struct {
	active js.Value
	k      *wanix.K
}

func Activate(ch js.Value, k *wanix.K) *Service {
	reg := jsutil.Await(jsutil.Get("navigator.serviceWorker").Call("getRegistration"))
	if reg.IsUndefined() {
		swPath := "./sw.js"
		jsutil.Await(jsutil.Get("navigator.serviceWorker").Call("register", swPath, map[string]any{"type": "module"}))
		reg = jsutil.Await(jsutil.Get("navigator.serviceWorker.ready"))
	}

	d := &Service{
		active: reg.Get("active"),
		k:      k,
	}
	ch.Get("port2").Set("onmessage", js.FuncOf(d.handleMessage))

	reg.Get("active").Call("postMessage", map[string]any{"listen": ch.Get("port1")}, []any{ch.Get("port1")})
	return d
}

func (d *Service) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	fsys := fskit.MapFS{
		"ctl": internal.ControlFile(&cli.Command{
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
		"state": internal.FieldFile(d.active.Get("state").String()),
		// "err": internal.FieldFile(r.state, nil),
		// "fsys": internal.FieldFile(r.fs, nil),
	}
	return fs.Resolve(fsys, ctx, name)
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

func (d *Service) handleMessage(this js.Value, args []js.Value) interface{} {
	if args[0].Get("data").Get("request").IsUndefined() {
		return nil
	}
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
		headerKeys := js.Global().Get("Object").Call("keys", jsReq.Get("headers"))
		headerKeys.Call("forEach", js.FuncOf(func(this js.Value, args []js.Value) any {
			req.Header.Add(args[0].String(), jsReq.Get("headers").Get(args[0].String()).String())
			return nil
		}))
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

func (d *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("X-Service-Worker", r.Header.Get("X-Service-Worker"))
	w.Header().Add("Cross-Origin-Opener-Policy", "same-origin")
	w.Header().Add("Cross-Origin-Embedder-Policy", "require-corp")
	if strings.HasPrefix(r.URL.Path, "/:/") {
		path := strings.TrimPrefix(r.URL.Path, "/:/")

		entries, err := fs.ReadDir(d.k.NS, strings.TrimSuffix(path, "/"))
		if err == nil {
			if !strings.HasSuffix(r.URL.Path, "/") {
				http.Redirect(w, r, r.URL.Path+"/", http.StatusTemporaryRedirect)
				return
			}
			for _, entry := range entries {
				if entry.Name() == "index.html" {
					f, err := d.k.NS.Open(filepath.Join(path, entry.Name()))
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					defer f.Close()
					http.ServeContent(w, r, entry.Name(), time.Now(), &nopReadSeeker{File: f})
					return
				}
			}
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		b, err := fs.ReadFile(d.k.NS, path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
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

type nopReadSeeker struct {
	fs.File
}

func (n *nopReadSeeker) Seek(offset int64, whence int) (int64, error) {
	// This is a no-op implementation that doesn't actually seek
	return 0, nil
}
