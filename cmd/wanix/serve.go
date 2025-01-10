package main

import (
	"log"
	"os"
	"os/signal"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs/memfs"
	"tractor.dev/wanix/fusekit"
)

func serveCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "serve",
		Short: "serve wanix",
		Run: func(ctx *cli.Context, args []string) {
			fsys := memfs.FS{
				"hello":             {Data: []byte("hello, world\n")},
				"fortune/k/ken.txt": {Data: []byte("If a program is too slow, it must have a loop.\n")},
			}

			mount, err := fusekit.Mount(fsys, "/tmp/wanix")
			if err != nil {
				log.Fatalf("Mount fail: %v\n", err)
			}
			defer func() {
				if err := mount.Close(); err != nil {
					log.Fatalf("Failed to unmount: %v\n", err)
				}
			}()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan)
			for sig := range sigChan {
				if sig == os.Interrupt {
					return
				}
			}
		},
	}
	return cmd
}
