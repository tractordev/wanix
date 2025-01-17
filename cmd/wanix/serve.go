package main

import (
	"context"
	"io/fs"
	"log"
	"os"
	"os/signal"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fusekit"
	"tractor.dev/wanix/kernel/fsys"
	"tractor.dev/wanix/kernel/proc"
)

func serveCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "serve",
		Short: "serve wanix",
		Run: func(ctx *cli.Context, args []string) {
			log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

			kdev := fsys.New()
			kdev.Register("hellofs", func(args []string) (fs.FS, error) {
				log.Println("NEW HELLOFS:", args)
				return fskit.MapFS{"hellofile": fskit.RawNode([]byte("hello, world\n"))}, nil
			})

			kproc := proc.New()
			kproc.Register("ns", func(args []string) (fs.FS, error) {
				return nil, nil
			})

			p, err := kproc.Alloc("ns")
			if err != nil {
				log.Fatal(err)
			}

			kroot := fskit.MapFS{
				"#dev":  kdev,
				"#proc": kproc,
			}

			nsfs := p.Namespace()
			if err := nsfs.Bind(kroot, ".", ".", ""); err != nil {
				log.Fatalf("kernel bind fail: %v\n", err)
			}
			if err := nsfs.Bind(nsfs, "#dev", "dev", ""); err != nil {
				log.Fatalf("dev bind fail: %v\n", err)
			}
			if err := nsfs.Bind(nsfs, "#proc", "proc", ""); err != nil {
				log.Fatalf("proc bind fail: %v\n", err)
			}

			mount, err := fusekit.Mount(nsfs, "/tmp/wanix", proc.NewContextWithPID(context.Background(), p.ID()))
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
