//go:build !bundle

package main

import (
	"log"
	"net/http"
	"os"
	"time"
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
	log.Println("Serving WANIX dev server at http://localhost:8080 ...")
	mux := http.NewServeMux()
	mux.Handle("/~dev/", http.StripPrefix("/~dev", http.FileServer(http.Dir(dir))))
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
	http.ListenAndServe(":8080", loggerMiddleware(mux))
}
