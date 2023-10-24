package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"syscall/js"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"tractor.dev/toolkit-go/engine/fs/xformfs"
	"tractor.dev/wanix/internal/jsutil"
)

type Gateway struct{}

func (s *Gateway) InitializeJS() {
	// expose to host
	js.Global().Get("sys").Call("handle", "web.request", map[string]any{
		"respondRPC": js.FuncOf(s.request),
	})
}

func (s *Gateway) request(this js.Value, args []js.Value) any {
	var (
		responder = args[0]
		call      = args[1]
	)
	return jsutil.Promise(func() (any, error) {
		params := jsutil.Await(call.Call("receive"))
		var (
			method  = params.Index(0).String()
			fullURL = params.Index(1).String()
		)

		u, _ := url.Parse(fullURL)
		req, err := http.NewRequest(method, u.Path, nil)
		if err != nil {
			fmt.Println("err:", err)
			return nil, err
		}
		rr := httptest.NewRecorder()

		// fmt.Println("SW", fullURL)
		s.ServeHTTP(rr, req)

		headers := make(map[string]any)
		for k, v := range rr.Result().Header {
			headers[k] = v[0]
		}

		ch := jsutil.Await(responder.Call("continue", headers))
		chw := &jsutil.Writer{ch}

		if _, err := rr.Body.WriteTo(chw); err != nil {
			fmt.Println("gateway:", err)
		}

		ch.Call("close")
		return nil, nil
	})
}

func (s *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, ok := map[string]bool{
		".js":  true,
		".jsx": true,
		".ts":  true,
		".tsx": true,
	}[filepath.Ext(r.URL.Path)]; ok {
		w.Header().Set("content-type", "text/javascript")
	}

	httpfs := xformfs.New(os.DirFS("."))
	httpfs.Transform(".jsx", TransformJSX)
	httpfs.Transform(".tsx", TransformTSX)
	httpfs.Transform(".ts", TransformTSX)

	http.FileServer(http.FS(httpfs)).ServeHTTP(w, r)
}

func TransformTSX(dst io.Writer, src io.Reader) error {
	b, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	result := esbuild.Transform(string(b), esbuild.TransformOptions{
		Loader:      esbuild.LoaderTSX,
		JSXFactory:  "m",
		JSXFragment: "",
	})
	if len(result.Errors) > 0 {
		fmt.Println(result.Errors)
		return fmt.Errorf("TSX transform errors")
	}
	_, err = dst.Write(append([]byte("\n"), result.Code...))
	return err
}

func TransformJSX(dst io.Writer, src io.Reader) error {
	b, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	result := esbuild.Transform(string(b), esbuild.TransformOptions{
		Loader:      esbuild.LoaderJSX,
		JSXFactory:  "m",
		JSXFragment: "",
	})
	if len(result.Errors) > 0 {
		fmt.Println(result.Errors)
		return fmt.Errorf("JSX transform errors")
	}
	_, err = dst.Write(append([]byte("\n"), result.Code...))
	return err
}
