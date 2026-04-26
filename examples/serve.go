package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("serving examples at http://localhost:7070/examples")
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Enable cross-origin isolation, needed for SharedArrayBuffer/WebAssembly threading/etc.
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
		http.FileServer(http.Dir(".")).ServeHTTP(w, r)
	}))
	if err := http.ListenAndServe(":7070", nil); err != nil {
		log.Fatal(err)
	}
}
