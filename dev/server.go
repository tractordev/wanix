//go:build !bundle

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

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
		gwutil.FileTransformer(os.DirFS(dir), httpfs.FileServer).ServeHTTP(w, r)
	})))
	mux.Handle(fmt.Sprintf("%s/wanix-bootloader.js", basePath), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/javascript")

		if os.Getenv("PROD") != "1" {
			http.ServeFile(w, r, "./dev/bootloader.js")
			return
		}

		// emulate a build
		PackFilesTo(w)
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
