package main

import (
	"fmt"
	"os"
	"path/filepath"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/toolkit-go/engine/fs/watchfs"
	"tractor.dev/wanix/internal/jsutil"
)

func openCmd() *cli.Command {
	var openWatch *watchfs.Watch
	cmd := &cli.Command{
		Usage: "open <appname>",
		Args:  cli.ExactArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			var path string
			var searchPaths = []string{"sys/app", "app", "sys/dev/internal/app"}
			for _, searchPath := range searchPaths {
				appPath := filepath.Join(searchPath, args[0])
				if exists, _ := fs.Exists(os.DirFS("/"), appPath); exists {
					path = appPath
					break
				}
			}
			if path == "" {
				fmt.Fprintln(ctx, "app not found")
				return
			}

			if openWatch != nil {
				openWatch.Close()
			}

			// todo: port from afero to engine/fs so watchfs works
			// --
			// var err error
			// var firstWrite bool
			// if args[0] == "jazz-todo" {
			// 	openWatch, err = watchfs.WatchFile(fs, "app/jazz-todo/view.jsx", &watchfs.Config{
			// 		Handler: func(e watchfs.Event) {
			// 			if e.Type == watchfs.EventWrite && len(e.Path) > len(path) {
			// 				if !firstWrite {
			// 					firstWrite = true
			// 					return
			// 				}
			// 				js.Global().Get("wanix").Get("loadApp").Invoke("main")
			// 			}
			// 		},
			// 	})
			// } else {
			// openWatch, err = watchfs.WatchFile(fs, path, &watchfs.Config{
			// 	Recursive: true,
			// 	Handler: func(e watchfs.Event) {
			// 		if e.Type == watchfs.EventWrite && len(e.Path) > len(path) {
			// 			if !firstWrite {
			// 				firstWrite = true
			// 				return
			// 			}
			// 			jsutil.WanixSyscall("host.loadApp", "main", path, true)
			// 		}
			// 	},
			// })
			// }
			// if err != nil {
			// 	fmt.Fprintln(ctx, err)
			// 	return
			// }

			jsutil.WanixSyscall("host.loadApp", "main", path, true)
		},
	}
	return cmd
}
