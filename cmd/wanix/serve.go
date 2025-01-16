package main

import (
	"context"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"strings"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fusekit"
	"tractor.dev/wanix/kernel/fsys"
	"tractor.dev/wanix/kernel/ns"
	"tractor.dev/wanix/kernel/proc"
)

func serveCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "serve",
		Short: "serve wanix",
		Run: func(ctx *cli.Context, args []string) {
			log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

			devfs := fsys.New()
			devfs.Register("hellofs", func(args []string) (fs.FS, error) {
				log.Println("NEW HELLOFS:", args)
				return fskit.MapFS{"hellofile": fskit.RawNode([]byte("hello, world\n"))}, nil
			})

			procfs := proc.New()
			procfs.Register("ns", func(args []string) (fs.FS, error) {
				return nil, nil
			})

			nsfs := ns.New()
			if err := nsfs.Bind(devfs, ".", "dev", "replace"); err != nil {
				log.Fatalf("devfs bind fail: %v\n", err)
			}
			if err := nsfs.Bind(procfs, ".", "proc", "replace"); err != nil {
				log.Fatalf("procfs bind fail: %v\n", err)
			}

			b, err := fs.ReadFile(procfs, "new/ns")
			if err != nil {
				log.Fatalf("ReadFile fail: %v\n", err)
			}
			fsctx := proc.NewContextWithPID(context.Background(), strings.TrimSpace(string(b)))

			mount, err := fusekit.Mount(nsfs, "/tmp/wanix", fsctx)
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
