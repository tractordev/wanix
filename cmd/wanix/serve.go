package main

import (
	"log"
	"testing/fstest"

	"github.com/hanwen/go-fuse/v2/fs"
	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fusekit"
)

func serveCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "serve",
		Short: "serve wanix",
		Run: func(ctx *cli.Context, args []string) {
			fsys := fstest.MapFS{
				"hello":             {Data: []byte("hello, world\n")},
				"fortune/k/ken.txt": {Data: []byte("If a program is too slow, it must have a loop.\n")},
			}

			server, err := fs.Mount("/tmp/wanix", &fusekit.Node{FS: fsys}, &fs.Options{})
			if err != nil {
				log.Fatalf("Mount fail: %v\n", err)
			}
			server.Wait()
		},
	}
	return cmd
}
