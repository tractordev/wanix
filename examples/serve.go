package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("serving examples at http://localhost:7070/examples")
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Enable cross-origin isolation, needed for SharedArrayBuffer/WebAssembly threading/etc.
		if r.Host == "localhost:7071" || r.Host == "127.0.0.1:7071" {
			w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
		}

		http.FileServer(http.Dir(".")).ServeHTTP(w, r)
	}))
	go func() {
		// also serve on :7071 for cross origin examples
		if err := http.ListenAndServe(":7071", nil); err != nil {
			log.Fatal(err)
		}
	}()
	if err := http.ListenAndServe(":7070", nil); err != nil {
		log.Fatal(err)
	}
}
