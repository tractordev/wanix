//go:build !js && !wasm

package main

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/external/linux"
	v86 "tractor.dev/wanix/external/v86"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/shell"
	"tractor.dev/wanix/wasm/assets"
)

func serveCmd() *cli.Command {
	var (
		listenAddr string
	)
	cmd := &cli.Command{
		Usage: "serve",
		Short: "serve wanix",
		Run: func(ctx *cli.Context, args []string) {
			log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)

			h, p, _ := net.SplitHostPort(listenAddr)
			if h == "" {
				h = "localhost"
			}
			fmt.Printf("serving on http://%s:%s ...\n", h, p)

			fsys := fskit.UnionFS{assets.Dir, fskit.MapFS{
				"v86":   v86.Dir,
				"linux": linux.Dir,
				"shell": shell.Dir,
			}}

			http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("Cross-Origin-Opener-Policy", "same-origin")
				w.Header().Add("Cross-Origin-Embedder-Policy", "require-corp")

				if r.URL.Path == "/wanix.bundle.js" {
					w.Header().Add("Content-Type", "text/javascript")
					w.Write(assets.WanixBundle())
					return
				}

				http.FileServerFS(fsys).ServeHTTP(w, r)
			}))
			http.ListenAndServe(listenAddr, nil)
		},
	}
	cmd.Flags().StringVar(&listenAddr, "listen", ":7654", "addr to serve on")
	return cmd
}
