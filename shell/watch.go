package main

import (
	"fmt"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/wanix/internal/jsutil"
	"tractor.dev/wanix/internal/osfs"
)

var watches = make(map[string]int)

func watchCmd() *cli.Command {
	var recursive bool

	cmd := &cli.Command{
		Usage: "watch [-recursive] <path>",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			path := unixToFsPath(args[0])

			if exists, err := fs.Exists(osfs.New(), path); !exists {
				if !checkErr(ctx, err) {
					fmt.Printf("file or directory at path '%s' doesn't exist\n", absPath(path))
				}
				return
			}

			if _, exists := watches[path]; exists {
				fmt.Printf("path '%s' is already being watched\n", absPath(path))
				return
			}

			// watch(path, recursive, eventMask, ignores)
			w, err := jsutil.WanixSyscall("fs.watch", path, recursive, 0, []any{})
			if err != nil {
				fmt.Fprintln(ctx, err)
				return
			}
			watches[path] = w.Int()
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
			path := unixToFsPath(args[0])
			wHandle, exists := watches[path]
			if !exists {
				fmt.Printf("path '%s' isn't being watched\n", absPath(path))
				return
			}

			_, err := jsutil.WanixSyscall("fs.unwatch", wHandle)
			if err != nil {
				fmt.Fprintln(ctx, err)
				return
			}

			delete(watches, path)
		},
	}
	return cmd
}
