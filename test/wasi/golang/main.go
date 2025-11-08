package main

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"
)

func main() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)

	wd, _ := os.Getwd()
	fmt.Fprintf(w, "Dir:\t%s\n", wd)

	fmt.Fprintf(w, "Args:\t%v\n", os.Args)

	fmt.Fprintf(w, "Env:\t%v\n", os.Environ())

	entries, err := os.ReadDir("/")
	if err != nil {
		log.Fatal(err)
		return
	}
	var root []string
	for _, entry := range entries {
		if entry.IsDir() {
			root = append(root, entry.Name()+"/")
		} else {
			root = append(root, entry.Name())
		}
	}
	fmt.Fprintf(w, "Root:\t%v\n", root)

	w.Flush()
}
