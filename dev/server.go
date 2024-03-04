//go:build !bundle

package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"tractor.dev/toolkit-go/engine/fs/watchfs"
	"tractor.dev/wanix/internal/httpfs"
	"tractor.dev/wanix/kernel/web/gwutil"
)

func loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Call the next handler in the chain
		next.ServeHTTP(w, r)
		// Log the request details
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func main() {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	basePath := ""
	log.Printf("Serving WANIX dev server at http://localhost:7777%s ...\n", basePath)

	mux := http.NewServeMux()
	mux.Handle(fmt.Sprintf("%s/sys/dev/", basePath), http.StripPrefix(fmt.Sprintf("%s/sys/dev/", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gwutil.FileTransformer(watchfs.New(os.DirFS(dir)), httpfs.FileServer).ServeHTTP(w, r)
	})))
	mux.Handle(fmt.Sprintf("%s/wanix-kernel.gz", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/gzip")

		file, err := os.Open(filepath.Join(dir, "local/bin/kernel"))
		if err != nil {
			http.Error(w, "File not found.", http.StatusNotFound)
			return
		}
		defer file.Close()

		gzipWriter := gzip.NewWriter(w)
		defer gzipWriter.Close()

		if _, err := io.Copy(gzipWriter, file); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := gzipWriter.Flush(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))
	mux.Handle(fmt.Sprintf("%s/wanix-bootloader.js", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/javascript")

		var packMode PackMode
		if os.Getenv("PROD") == "1" {
			log.Printf("Packing self-contained bootloader...\n")
			packMode = PackFileData
		} else {
			packMode = PackFilePaths
		}

		// TODO: Does this need to pack on every request for the bootloader?
		// I don't think you want to be changing PROD at runtime, so we can
		// probably cache this.
		PackFilesTo(w, packMode)
		f, err := os.ReadFile("./dev/bootloader.js")
		fatal(err)
		w.Write(f)
	}))
	mux.Handle(fmt.Sprintf("%s/~init/", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	}))
	mux.Handle(fmt.Sprintf("%s/", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./dev/index.html")
	}))
	if err := http.ListenAndServe(":7777", loggerMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}
