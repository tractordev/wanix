//go:build !js && !wasm

package main

import (
	"log"
	"os"
	"os/signal"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix"
	"tractor.dev/wanix/fs/fusekit"
)

func mountCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "mount",
		Short: "mount wanix",
		Run: func(ctx *cli.Context, args []string) {
			log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

			k := wanix.New()

			root, err := k.NewRoot()
			fatal(err)

			root.Bind("#cap", "cap")
			root.Bind("#task", "task")

			mount, err := fusekit.Mount(root.Namespace(), "/tmp/wanix", root.Context())
			fatal(err)
			defer func() {
				fatal(mount.Close())
			}()

			log.Println("Mounted at /tmp/wanix ...")

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
