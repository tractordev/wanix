package gwutil

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"sync"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"tractor.dev/toolkit-go/engine/fs/xformfs"
)

var mu sync.Mutex

func FileTransformer(fsys fs.FS, fserver func(fs.FS) http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := map[string]bool{
			".js":  true,
			".jsx": true,
			".ts":  true,
			".tsx": true,
		}[filepath.Ext(r.URL.Path)]; ok {
			w.Header().Set("content-type", "text/javascript")
		}

		httpfs := xformfs.New(fsys)
		httpfs.Transform(".jsx", TransformJSX)
		httpfs.Transform(".tsx", TransformTSX)
		httpfs.Transform(".ts", TransformTSX)

		fserver(httpfs).ServeHTTP(w, r)
	})
}

func TransformTSX(dst io.Writer, src io.Reader) error {
	b, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	mu.Lock()
	result := esbuild.Transform(string(b), esbuild.TransformOptions{
		Loader:      esbuild.LoaderTSX,
		JSXFactory:  "m",
		JSXFragment: "",
	})
	mu.Unlock()
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
	mu.Lock()
	result := esbuild.Transform(string(b), esbuild.TransformOptions{
		Loader:      esbuild.LoaderJSX,
		JSXFactory:  "m",
		JSXFragment: "",
	})
	mu.Unlock()
	if len(result.Errors) > 0 {
		fmt.Println(result.Errors)
		return fmt.Errorf("JSX transform errors")
	}
	_, err = dst.Write(append([]byte("\n"), result.Code...))
	return err
}
