package main

import (
	"embed"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"tractor.dev/toolkit-go/engine/fs/xformfs"
)

//go:embed assets
var assets embed.FS

func main() {
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := map[string]bool{
			".js":  true,
			".jsx": true,
			".ts":  true,
			".tsx": true,
		}
		if _, ok := ext[filepath.Ext(r.URL.Path)]; ok {
			w.Header().Set("content-type", "text/javascript")
		}
		xfs := xformfs.New(assets)
		xfs.Transform(".jsx", transformJSX)
		xfs.Transform(".tsx", transformTSX)
		xfs.Transform(".ts", transformTSX)
		http.FileServer(http.FS(xfs)).ServeHTTP(w, r)
	}))
	fmt.Println("Serving at http://localhost:7070/ ...")
	http.ListenAndServe(":7070", nil)
}

func transformTSX(dst io.Writer, src io.Reader) error {
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

func transformJSX(dst io.Writer, src io.Reader) error {
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
