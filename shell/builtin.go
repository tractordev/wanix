package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall/js"

	"github.com/spf13/afero"
	"github.com/spf13/afero/mem"
	"golang.org/x/term"
	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs/watchfs"
)

func echoCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "echo [text]...",
		Run: func(ctx *cli.Context, args []string) {
			io.WriteString(ctx, strings.Join(args, " "))
		},
	}
	return cmd
}

func openCmd() *cli.Command {
	var openWatch *watchfs.Watch
	cmd := &cli.Command{
		Usage: "open <path>",
		Args:  cli.ExactArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			var path string
			if exists, _ := afero.Exists(afero.NewOsFs(), fmt.Sprintf("app/%s", args[0])); exists {
				path = fmt.Sprintf("app/%s", args[0])
			}
			if exists, _ := afero.Exists(afero.NewOsFs(), fmt.Sprintf("sys/app/%s", args[0])); exists {
				path = fmt.Sprintf("sys/app/%s", args[0])
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
			js.Global().Get("wanix").Get("loadApp").Invoke("main", fmt.Sprintf("/-/%s/", path), true)
		},
	}
	return cmd
}

func mtimeCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "mtime <path>",
		Args:  cli.ExactArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			fi, err := os.Stat(args[0])
			if err != nil {
				fmt.Fprintf(ctx, "%s\n", err)
				return
			}
			fmt.Fprintf(ctx, "%s\n", fi.ModTime())
		},
	}
	return cmd
}

func lsCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "ls [path]",
		Args:  cli.MaxArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			var path string
			if len(args) > 0 {
				path = absPath(args[0])
			} else {
				path, _ = os.Getwd()
			}

			fi, err := os.ReadDir(path)
			if checkErr(ctx, err) {
				return
			}
			for _, entry := range fi {
				dirSuffix := ' '
				if entry.IsDir() {
					dirSuffix = '/'
				}
				info, _ := entry.Info()
				fmt.Fprintf(ctx, "%v %-4d %s%c\n", info.Mode(), info.Size(), info.Name(), dirSuffix)
			}
		},
	}
	return cmd
}

func cdCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "cd <path>",
		Args:  cli.ExactArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			fsPath := unixToFsPath(args[0])

			exists, err := afero.DirExists(afero.NewOsFs(), fsPath)
			if checkErr(ctx, err) {
				return
			}

			if !exists {
				fmt.Fprintln(ctx, "invalid directory")
				return
			}

			path := absPath(args[0])
			if path == "." {
				return
			}
			if checkErr(ctx, os.Chdir(path)) {
				return
			}

		},
	}
	return cmd
}

func catCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "cat <path>...",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			// todo: multiple files
			d, err := os.ReadFile(unixToFsPath(args[0]))
			if checkErr(ctx, err) {
				return
			}
			ctx.Write(d)
			io.WriteString(ctx, "\n")
		},
	}
	return cmd
}

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

// deprecated
type Command func(t *term.Terminal, fs afero.Fs, args []string)

// deprecated
var commands map[string]Command

func initCommands() {
	// TODO: port these to cli.Commands like above
	commands = map[string]Command{
		"reload": func(t *term.Terminal, fs afero.Fs, args []string) {
			js.Global().Get("wanix").Get("reload").Invoke()
		},
		"dl": func(t *term.Terminal, fs afero.Fs, args []string) {
			js.Global().Get("wanix").Get("download").Invoke(args[0])
		},
		// for debugging
		"resetfs": func(t *term.Terminal, _ afero.Fs, args []string) {
			//fs.Reset(nil)
		},
		"fsdata": func(t *term.Terminal, fs afero.Fs, args []string) {
			// watchFS := fs.(*watchfs.FS)
			// watched := GetUnexportedField(reflect.ValueOf(watchFS).Elem().FieldByIndex([]int{0}))
			cowFS := fs.(*afero.CopyOnWriteFs)
			layer := GetUnexportedField(reflect.ValueOf(cowFS).Elem().FieldByName("layer"))
			memFS := layer.(*afero.MemMapFs)
			data := GetUnexportedField(reflect.ValueOf(memFS).Elem().FieldByName("data"))
			fdata := data.(map[string]*mem.FileData)

			for name, fd := range fdata {
				memDir := GetUnexportedField(reflect.ValueOf(fd).Elem().FieldByName("memDir"))
				fmt.Printf("%s:\nFileData:%+v\nDirMap:%+v\n", name, *fd, memDir)
			}
		},
		"touch": func(t *term.Terminal, fs afero.Fs, args []string) {
			err := afero.WriteFile(fs, unixToFsPath(args[0]), []byte{}, 0644)
			if checkErr(t, err) {
				return
			}
		},
		"cat": func(t *term.Terminal, fs afero.Fs, args []string) {
			d, err := afero.ReadFile(fs, unixToFsPath(args[0]))
			if checkErr(t, err) {
				return
			}
			t.Write(d)
			io.WriteString(t, "\n")
		},
		"rm": func(t *term.Terminal, fs afero.Fs, args []string) {
			if args[0] == "-r" {
				// TODO: doesn't return an error if path doesn't exist
				err := fs.RemoveAll(unixToFsPath(args[1]))
				if checkErr(t, err) {
					return
				}
			} else {
				path := unixToFsPath(args[0])
				if isdir, _ := afero.IsDir(fs, path); isdir {
					fmt.Fprintf(t, "cant remove file %s: is a directory\n(try using the `-r` flag)\n", absPath(args[0]))
				}
				// TODO: fs.Remove doesn't error if you pass it a directory!
				// it also gives the wrong error if trying to delete a readonly file,
				// (should be Operation not permitted)
				err := fs.Remove(path)
				if checkErr(t, err) {
					return
				}
			}
		},
		"mkdir": func(t *term.Terminal, fs afero.Fs, args []string) {
			err := fs.Mkdir(unixToFsPath(args[0]), 0755)
			if checkErr(t, err) {
				return
			}
		},
		// Move SOURCE to DEST, or multiple SOURCE(s) to DIRECTORY.
		"mv": func(t *term.Terminal, fs afero.Fs, args []string) {
			// TODO: prevent file overwrite if dest file already exits (should this already be an error?)
			// TODO: error when dest directory doesn't exist and args.len > 2
			isdir, err := afero.DirExists(fs, unixToFsPath(args[len(args)-1]))
			if checkErr(t, err) {
				return
			}
			if isdir {
				// move all paths into this directory
				dir := absPath(args[len(args)-1])
				for _, path := range args[:len(args)-1] {
					src := filepath.Base(path)
					dest := filepath.Join(dir, src)
					err := fs.Rename(unixToFsPath(path), unixToFsPath(dest))
					if err != nil {
						io.WriteString(t, fmt.Sprintln(err))
						continue
					}
				}
			} else {
				err := fs.Rename(unixToFsPath(args[0]), unixToFsPath(args[1]))
				if checkErr(t, err) {
					return
				}
			}
		},
		// Copy SOURCE to DEST, or multiple SOURCE(s) to DIRECTORY.
		"cp": func(t *term.Terminal, fs afero.Fs, args []string) {
			// TODO: handle copying directories
			isdir, err := afero.DirExists(fs, unixToFsPath(args[len(args)-1]))
			if checkErr(t, err) {
				return
			}
			if isdir {
				// copy all paths to this directory
				dir := absPath(args[len(args)-1])
				recursive := args[0] == "-r"

				start := 0
				if recursive {
					start = 1
				}

				for _, path := range args[start : len(args)-1] {
					srcName := filepath.Base(path)
					dest := filepath.Join(dir, srcName)

					srcIsDir, err := afero.IsDir(fs, unixToFsPath(path))
					if checkErr(t, err) {
						continue
					}

					if srcIsDir {
						if !recursive {
							io.WriteString(t, fmt.Sprintf("-r not specified; omitting directory '%s'\n", path))
							continue
						}

						err = fs.MkdirAll(unixToFsPath(dest), 0755)
						if checkErr(t, err) {
							continue
						}

						entries, err := afero.ReadDir(fs, unixToFsPath(path))
						if checkErr(t, err) {
							continue
						}

						for _, e := range entries {
							commands["cp"](t, fs, []string{"-r", filepath.Join(path, e.Name()), dest})
						}
					} else {
						content, err := afero.ReadFile(fs, unixToFsPath(path))
						if checkErr(t, err) {
							continue
						}
						err = afero.WriteFile(fs, unixToFsPath(dest), content, 0644)
						if checkErr(t, err) {
							continue
						}
					}
				}
			} else {
				content, err := afero.ReadFile(fs, unixToFsPath(args[0]))
				if checkErr(t, err) {
					return
				}

				err = afero.WriteFile(fs, unixToFsPath(args[1]), content, 0644)
				if checkErr(t, err) {
					return
				}
			}
		},
		"pwd": func(t *term.Terminal, fs afero.Fs, args []string) {
			wd, _ := os.Getwd()
			io.WriteString(t, fmt.Sprintln(wd))
		},
		"write": func(t *term.Terminal, fs afero.Fs, args []string) {
			afero.WriteFile(fs, args[0], []byte(strings.Join(args[1:], " ")), 0644)
		},
		"env": func(t *term.Terminal, fs afero.Fs, args []string) {
			for _, kvp := range os.Environ() {
				fmt.Fprintln(t, kvp)
			}
		},
		// export [-remove] <NAME[=VALUE]>...
		"export": func(t *term.Terminal, fs afero.Fs, args []string) {
			remove := args[0] == "-remove"
			if remove {
				args = args[1:]
			}

			for i, arg := range args {
				name, value, _ := strings.Cut(arg, "=")
				if name == "" {
					io.WriteString(t, fmt.Sprintf("invalid argument (%d): missing variable name", i))
					return
				}
				if remove {
					os.Setenv(name, "")
				} else {
					os.Setenv(name, value)
				}
			}
		},
	}
}
