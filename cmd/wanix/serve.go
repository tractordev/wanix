package main

import (
	"fmt"
	"net/http"

	"tractor.dev/toolkit-go/engine/cli"
	v86 "tractor.dev/wanix/external/v86"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/wasm/assets"
)

func serveCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "serve",
		Short: "serve wanix",
		Run: func(ctx *cli.Context, args []string) {
			fmt.Println("serving on http://localhost:8080 ...")

			fsys := fskit.UnionFS{assets.Dir, fskit.MapFS{
				"v86": v86.Dir,
			}}

			http.ListenAndServe(`:8080`, http.FileServerFS(fsys))
		},
	}
	return cmd
}
