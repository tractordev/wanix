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
			if err != nil {
				log.Fatal(err)
			}

			root.Bind("#fsys", "fsys")
			root.Bind("#proc", "proc")

			mount, err := fusekit.Mount(root.Namespace(), "/tmp/wanix", root.Context())
			if err != nil {
				log.Fatalf("Mount fail: %v\n", err)
			}
			defer func() {
				if err := mount.Close(); err != nil {
					log.Fatalf("Failed to unmount: %v\n", err)
				}
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
