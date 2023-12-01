package main

// todo: port from afero to engine/fs so watchfs works
// --
// var watches = make(map[string]*watchfs.Watch)
// func watchCmd() *cli.Command {
// 	cmd := &cli.Command{
// 		Usage: "watch <path>", // todo add -r
// 		Args:  cli.ExactArgs(1),
// 		Run: func(ctx *cli.Context, args []string) {
// 			var recursive bool
// 			var path string
// 			if args[0] == "-r" {
// 				recursive = true
// 				path = args[1]
// 			} else {
// 				path = args[0]
// 			}
// 			if _, exists := watches[path]; exists {
// 				return
// 			}
// 			w, err := watchfs.WatchFile(fs, path, &watchfs.Config{
// 				Recursive: recursive,
// 				Handler: func(e watchfs.Event) {
// 					js.Global().Get("console").Call("log", e.String())
// 				},
// 			})
// 			if err != nil {
// 				fmt.Fprintf(t, "%s\n", err)
// 				return
// 			}
// 			watches[args[0]] = w
// 		},
// 	}
// 	return cmd
// }
// func unwatchCmd() *cli.Command {
// 	cmd := &cli.Command{
// 		Usage: "unwatch <path>", // todo add -r
// 		Args:  cli.ExactArgs(1),
// 		Run: func(ctx *cli.Context, args []string) {
// 			w, exists := watches[args[0]]
// 			if !exists {
// 				return
// 			}
// 			w.Close()
// 			delete(watches, args[0])
// 			go func() {
// 				for e := range w.Iter() {
// 					js.Global().Get("console").Call("log", e.String())
// 				}
// 			}()
// 		},
// 	}
// 	return cmd
// }
