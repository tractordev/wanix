package main

import (
	"fmt"
	"os"
	"syscall/js"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/toolkit-go/engine/fs/watchfs"
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
				if exists, _ := fs.Exists(os.DirFS("/"), fmt.Sprintf("%s/%s", searchPath, args[0])); exists {
					path = fmt.Sprintf("%s/%s", searchPath, args[0])
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
			// 	openWatch, err = watchfs.WatchFile(fs, path, &watchfs.Config{
			// 		Recursive: true,
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
			// }
			// if err != nil {
			// 	fmt.Fprintf(t, "%s\n", err)
			// 	return
			// }
			js.Global().Get("sys").Call("call", "host.loadApp", []any{"main", path, true})
		},
	}
	return cmd
}
