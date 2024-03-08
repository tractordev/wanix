package main

// Commands in this file should be small, <~50 loc. Bigger ones should get a
// dedicated file. Use common sense; if it takes up your whole screen, move it.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/wanix/internal/jsutil"
)

func exitCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "exit",
		Run: func(ctx *cli.Context, args []string) {
			os.Exit(0)
		},
	}
	return cmd
}

func helpCmd(root *cli.Command) *cli.Command {
	cmd := &cli.Command{
		Usage: "help",
		Run: func(ctx *cli.Context, args []string) {
			(&cli.CommandHelp{root}).WriteHelp(ctx)
		},
	}
	return cmd
}

func echoCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "echo [text]...",
		Run: func(ctx *cli.Context, args []string) {
			io.WriteString(ctx, strings.Join(args, " "))
		},
	}
	return cmd
}

func mtimeCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "mtime <path>",
		Args:  cli.ExactArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			fi, err := os.Stat(absPath(args[0]))
			if err != nil {
				fmt.Fprintln(ctx, err)
				return
			}
			fmt.Fprintln(ctx, fi.ModTime())
		},
	}
	return cmd
}

func inviteCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "invite",
		Run: func(ctx *cli.Context, args []string) {
			ret, err := jsutil.WanixSyscall("jazz.invite")
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println(ret.String())
		},
	}
	return cmd
}

func loginCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "login",
		Run: func(ctx *cli.Context, args []string) {
			jsutil.WanixSyscall("host.login")
		},
	}
	return cmd
}

func logoutCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "logout",
		Run: func(ctx *cli.Context, args []string) {
			jsutil.WanixSyscall("host.logout")
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
			exists, err := fs.DirExists(os.DirFS("/"), unixToFsPath(args[0]))
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
			d, err := os.ReadFile(absPath(args[0]))
			if checkErr(ctx, err) {
				return
			}
			ctx.Write(d)
			io.WriteString(ctx, "\n")
		},
	}
	return cmd
}

func reloadCmd() *cli.Command {
	return &cli.Command{
		Usage: "reload",
		Args:  cli.ExactArgs(0),
		Run: func(ctx *cli.Context, args []string) {
			fmt.Println("TODO: Unimplemented")
			// js.Global().Get("wanix").Get("reload").Invoke()
		},
	}
}

func touchCmd() *cli.Command {
	return &cli.Command{
		Usage: "touch <path>...",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			// TODO: multiple files, options for updating a/mtimes
			err := os.WriteFile(absPath(args[0]), []byte{}, 0644)
			if checkErr(ctx, err) {
				return
			}
		},
	}
}

func removeCmd() *cli.Command {
	var recursive bool

	cmd := &cli.Command{
		Usage: "rm [-r] <path>...",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			for _, arg := range args {
				if recursive {
					err := os.RemoveAll(absPath(arg))
					if checkErr(ctx, err) {
						return
					}
				} else {
					isdir, err := fs.IsDir(os.DirFS("/"), unixToFsPath(arg))
					if checkErr(ctx, err) {
						return
					}
					if isdir {
						fmt.Fprintf(ctx, "Can't remove file %s: is a directory\n(try using the `-r` flag)\n", absPath(arg))
						continue
					}

					err = os.Remove(absPath(arg))
					if checkErr(ctx, err) {
						return
					}
				}
			}
		},
	}

	cmd.Flags().BoolVar(&recursive, "r", false, "Remove recursively")
	return cmd
}

func mkdirCmd() *cli.Command {
	return &cli.Command{
		Usage: "mkdir <path>",
		Args:  cli.ExactArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			// TODO: support MkdirAll
			err := os.Mkdir(absPath(args[0]), 0755)
			if checkErr(ctx, err) {
				return
			}
		},
	}
}

func moveCmd() *cli.Command {
	var overwrite bool

	cmd := &cli.Command{
		Usage: "mv [-overwrite] SOURCE DEST | SOURCE... DIRECTORY",
		Args:  cli.MinArgs(2),
		Short: "Rename SOURCE to DEST, or move SOURCE(s) to DIRECTORY.",
		Run: func(ctx *cli.Context, args []string) {
			dfs := os.DirFS("/")
			isdir, err := fs.DirExists(dfs, unixToFsPath(args[len(args)-1]))
			if checkErr(ctx, err) {
				return
			}
			if isdir {
				// move all paths into this directory
				dir := absPath(args[len(args)-1])
				for _, path := range args[:len(args)-1] {
					src := absPath(path)
					dest := filepath.Join(dir, filepath.Base(src))
					destExists, err := fs.Exists(dfs, unixToFsPath(dest))
					if checkErr(ctx, err) {
						return
					}

					if destExists && !overwrite {
						fmt.Fprintf(ctx, "Destination '%s' already exists. (Try using the `-overwrite` flag) Skipping...\n", dest)
						continue
					}

					err = os.Rename(src, dest)
					if checkErr(ctx, err) {
						continue
					}
				}
			} else {
				if len(args) > 2 {
					io.WriteString(ctx, "Cannot rename multiple sources to a single path. Did you mean to move them to a directory instead?\n")
					return
				}
				err := os.Rename(absPath(args[0]), absPath(args[1]))
				if checkErr(ctx, err) {
					return
				}
			}
		},
	}

	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite files/directories at the destination if they already exist.")
	return cmd
}

func pwdCmd() *cli.Command {
	return &cli.Command{
		Usage: "pwd",
		Args:  cli.ExactArgs(0),
		Run: func(ctx *cli.Context, args []string) {
			wd, err := os.Getwd()
			if checkErr(ctx, err) {
				return
			}
			io.WriteString(ctx, fmt.Sprintln(wd))
		},
	}
}

func writeCmd() *cli.Command {
	return &cli.Command{
		Usage: "write <filepath> [text]...",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			input := append([]byte(strings.Join(args[1:], " ")), '\n')
			err := os.WriteFile(absPath(args[0]), input, 0644)
			if checkErr(ctx, err) {
				return
			}
		},
	}
}

func printEnvCmd() *cli.Command {
	return &cli.Command{
		Usage: "env",
		Args:  cli.ExactArgs(0),
		Run: func(ctx *cli.Context, args []string) {
			for _, kvp := range os.Environ() {
				fmt.Fprintln(ctx, kvp)
			}
		},
	}
}

func exportCmd() *cli.Command {
	var remove bool

	cmd := &cli.Command{
		Usage: "export [-remove] <NAME[=VALUE]>...",
		Args:  cli.MinArgs(1),
		Short: "Set or remove environment variables.",
		Run: func(ctx *cli.Context, args []string) {
			for i, arg := range args {
				name, value, _ := strings.Cut(arg, "=")
				if name == "" {
					io.WriteString(ctx, fmt.Sprintf("invalid argument (%d): missing variable name", i))
					return
				}
				if remove {
					os.Unsetenv(name)
				} else {
					os.Setenv(name, value)
				}
			}
		},
	}

	// weird but it works
	cmd.Flags().BoolVar(&remove, "remove", false, "Remove an environment variable")
	return cmd
}

// TODO: port these debug commands to cli.Commands like above
// "resetfs": func(t *term.Terminal, _ afero.Fs, args []string) {
// 	fs.Reset(nil)
// },
// "fsdata": func(t *term.Terminal, fs afero.Fs, args []string) {
// 	// watchFS := fs.(*watchfs.FS)
// 	// watched := GetUnexportedField(reflect.ValueOf(watchFS).Elem().FieldByIndex([]int{0}))
// 	cowFS := fs.(*afero.CopyOnWriteFs)
// 	layer := GetUnexportedField(reflect.ValueOf(cowFS).Elem().FieldByName("layer"))
// 	memFS := layer.(*afero.MemMapFs)
// 	data := GetUnexportedField(reflect.ValueOf(memFS).Elem().FieldByName("data"))
// 	fdata := data.(map[string]*mem.FileData)

// 	for name, fd := range fdata {
// 		memDir := GetUnexportedField(reflect.ValueOf(fd).Elem().FieldByName("memDir"))
// 		fmt.Printf("%s:\nFileData:%+v\nDirMap:%+v\n", name, *fd, memDir)
// 	}
// },
