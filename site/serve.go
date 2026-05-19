package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("serving site at http://localhost:7072/")
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// // Enable cross-origin isolation, needed for SharedArrayBuffer/WebAssembly threading/etc.
		// if r.Host == "localhost:7071" || r.Host == "127.0.0.1:7071" {
		// 	w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		// 	w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
		// }

		http.FileServer(http.Dir("./site")).ServeHTTP(w, r)
	}))
	if err := http.ListenAndServe(":7072", nil); err != nil {
		log.Fatal(err)
	}
}
