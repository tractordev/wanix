//go:build !js && !wasm

package main

import (
	"fmt"
	"net/http"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/external/linux"
	v86 "tractor.dev/wanix/external/v86"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/wasm/assets"
)

func serveCmd() *cli.Command {
	var (
		port string
	)
	cmd := &cli.Command{
		Usage: "serve",
		Short: "serve wanix",
		Run: func(ctx *cli.Context, args []string) {
			fmt.Printf("serving on http://localhost:%s ...\n", port)

			fsys := fskit.UnionFS{assets.Dir, fskit.MapFS{
				"v86":   v86.Dir,
				"linux": linux.Dir,
			}}

			http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("Cross-Origin-Opener-Policy", "same-origin")
				w.Header().Add("Cross-Origin-Embedder-Policy", "require-corp")
				http.FileServerFS(fsys).ServeHTTP(w, r)
			}))
			http.ListenAndServe(":"+port, nil)
		},
	}
	cmd.Flags().StringVar(&port, "port", "8080", "port to serve on")
	return cmd
}
