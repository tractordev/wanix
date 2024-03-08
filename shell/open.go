package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall/js"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/toolkit-go/engine/fs/watchfs"
	"tractor.dev/wanix/internal/jsutil"
)

var openWatchResp js.Value = js.Null()

func openCmd() *cli.Command {
	var static bool

	cmd := &cli.Command{
		Usage: "open [-static] <appname>",
		Args:  cli.ExactArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			var path string
			var searchPaths = []string{"sys/app", "app", "sys/dev/internal/app", "grp/app"}
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

			if !static {
				if !openWatchResp.IsNull() {
					// close rpc channel
					jsutil.Await(openWatchResp.Call("send", 0))
				}

				var err error

				// watch(path, recursive, eventMask, ignores)
				openWatchResp, err = jsutil.WanixSyscallResp("fs.watch", path, true, 0, []any{})
				if err != nil {
					fmt.Fprintln(ctx, err)
					return
				}

				go func() {
					for {
						event := jsutil.Await(openWatchResp.Call("receive"))
						if event.IsNull() {
							return
						}

						if watchfs.EventType(event.Get("type").Int()) == watchfs.EventWrite && len(event.Get("path").String()) > len(path) {
							// loadApp(target, path, focus)
							jsutil.WanixSyscall("host.loadApp", "main", path, true)
						}
					}
				}()
			}

			// loadApp(target, path, focus)
			jsutil.WanixSyscall("host.loadApp", "main", path, true)
		},
	}
	cmd.Flags().BoolVar(&static, "static", false, "open without live reloading")
	return cmd
}
