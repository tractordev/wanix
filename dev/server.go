//go:build !bundle

package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"tractor.dev/wanix/internal/httpfs"
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
	log.Println("Serving WANIX dev server at http://localhost:7777 ...")

	mux := http.NewServeMux()
	mux.Handle("/sys/dev/", http.StripPrefix("/sys/dev/", httpfs.FileServer(os.DirFS(dir))))
	mux.Handle("/wanix-bootloader.js", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	mux.Handle("/~init/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	}))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./dev/index.html")
	}))
	if err := http.ListenAndServe(":7777", loggerMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}
