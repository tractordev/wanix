package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("serving examples at http://localhost:7070/examples")
	http.Handle("/", http.FileServer(http.Dir(".")))
	if err := http.ListenAndServe(":7070", nil); err != nil {
		log.Fatal(err)
	}
}
