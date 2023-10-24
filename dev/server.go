package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Serving WANIX dev server at http://localhost:8080 ...")
	mux := http.NewServeMux()
	mux.Handle("/~dev/", http.StripPrefix("/~dev", http.FileServer(http.Dir(dir))))
	mux.Handle("/bootloader.dev.js", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/javascript")
		http.ServeFile(w, r, "./dev/bootloader.dev.js")
	}))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./dev/index.html")
	}))
	http.ListenAndServe(":8080", mux)
}
