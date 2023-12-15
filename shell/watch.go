package main

import (
	"fmt"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/toolkit-go/engine/fs/watchfs"
	"tractor.dev/wanix/internal/osfs"
)

var watcher *watchfs.FS
var watches = make(map[string]*watchfs.Watch)

func watchCmd() *cli.Command {
	var recursive bool

	cmd := &cli.Command{
		Usage: "watch [-recursive] <path>",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			path := absPath(args[0])

			if watcher == nil {
				watcher = watchfs.New(osfs.New())
			}

			if exists, err := fs.Exists(watcher, path); !exists {
				if !checkErr(ctx, err) {
					fmt.Printf("file or directory at path '%s' doesn't exist\n", path)
				}
				return
			}

			if _, exists := watches[path]; exists {
				fmt.Printf("path '%s' is already being watched\n", path)
				return
			}

			w, err := watcher.Watch(path, &watchfs.Config{
				Recursive: recursive,
				Handler: func(e watchfs.Event) {
					fmt.Println("event", e.String())
				},
			})
			if err != nil {
				fmt.Fprintln(ctx, err)
				return
			}
			watches[path] = w
		},
	}

	cmd.Flags().BoolVar(&recursive, "recursive", false, "")
	return cmd
}

func unwatchCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "unwatch <path>", // todo add -r
		Args:  cli.ExactArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			path := absPath(args[0])
			w, exists := watches[path]
			if !exists {
				fmt.Printf("path '%s' isn't being watched\n", path)
				return
			}
			w.Close()
			delete(watches, path)
			go func() {
				for e := range w.Iter() {
					fmt.Println("event", e.String())
				}
			}()
		},
	}
	return cmd
}
