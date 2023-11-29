package cmds

// Commands in this file should be small, <~50 loc. Bigger ones should get a
// dedicated file. Use common sense; if it takes up your whole screen, move it.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall/js"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
	. "tractor.dev/wanix/shell/internal/sharedutil"
)

func ExitCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "exit",
		Run: func(ctx *cli.Context, args []string) {
			os.Exit(0)
		},
	}
	return cmd
}

func EchoCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "echo [text]...",
		Run: func(ctx *cli.Context, args []string) {
			io.WriteString(ctx, strings.Join(args, " "))
		},
	}
	return cmd
}

func MtimeCmd() *cli.Command {
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

func LsCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "ls [path]",
		Args:  cli.MaxArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			var path string
			if len(args) > 0 {
				path = AbsPath(args[0])
			} else {
				path, _ = os.Getwd()
			}

			fi, err := os.ReadDir(path)
			if CheckErr(ctx, err) {
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

func CdCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "cd <path>",
		Args:  cli.ExactArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			exists, err := fs.DirExists(os.DirFS("/"), UnixToFsPath(args[0]))
			if CheckErr(ctx, err) {
				return
			}

			if !exists {
				fmt.Fprintln(ctx, "invalid directory")
				return
			}

			path := AbsPath(args[0])
			if path == "." {
				return
			}
			if CheckErr(ctx, os.Chdir(path)) {
				return
			}

		},
	}
	return cmd
}

func CatCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "cat <path>...",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			// todo: multiple files
			d, err := os.ReadFile(AbsPath(args[0]))
			if CheckErr(ctx, err) {
				return
			}
			ctx.Write(d)
			io.WriteString(ctx, "\n")
		},
	}
	return cmd
}

func ReloadCmd() *cli.Command {
	return &cli.Command{
		Usage: "reload",
		Args:  cli.ExactArgs(0),
		Run: func(ctx *cli.Context, args []string) {
			fmt.Println("TODO: Unimplemented")
			// js.Global().Get("wanix").Get("reload").Invoke()
		},
	}
}

func DownloadCmd() *cli.Command {
	return &cli.Command{
		Usage: "dl <path>",
		Args:  cli.ExactArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			fmt.Println("TODO: Unimplemented")
			// js.Global().Get("wanix").Get("download").Invoke(args[0])
		},
	}
}

func TouchCmd() *cli.Command {
	return &cli.Command{
		Usage: "touch <path>...",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			// TODO: multiple files, options for updating a/mtimes
			err := os.WriteFile(AbsPath(args[0]), []byte{}, 0644)
			if CheckErr(ctx, err) {
				return
			}
		},
	}
}

func RemoveCmd() *cli.Command {
	var recursive bool

	cmd := &cli.Command{
		Usage: "rm [-r] <path>...",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			// TODO: multiple files
			if recursive {
				err := os.RemoveAll(AbsPath(args[0]))
				if CheckErr(ctx, err) {
					return
				}
			} else {
				if isdir, err := fs.IsDir(os.DirFS("/"), UnixToFsPath(args[0])); isdir {
					fmt.Fprintf(ctx, "cant remove file %s: is a directory\n(try using the `-r` flag)\n", AbsPath(args[0]))
					return
				} else if CheckErr(ctx, err) {
					return
				}

				// TODO: fs.Remove gives the wrong error if trying to delete a readonly file,
				// (should be Operation not permitted)
				err := os.Remove(AbsPath(args[0]))
				if CheckErr(ctx, err) {
					return
				}
			}
		},
	}

	cmd.Flags().BoolVar(&recursive, "r", false, "Remove recursively")
	return cmd
}

func MkdirCmd() *cli.Command {
	return &cli.Command{
		Usage: "mkdir <path>",
		Args:  cli.ExactArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			// TODO: support MkdirAll
			err := os.Mkdir(AbsPath(args[0]), 0755)
			if CheckErr(ctx, err) {
				return
			}
		},
	}
}

func MoveCmd() *cli.Command {
	return &cli.Command{
		Usage: "mv SOURCE DEST | SOURCE... DIRECTORY",
		Args:  cli.MinArgs(2),
		Short: "Rename SOURCE to DEST, or move multiple SOURCE(s) to DIRECTORY.",
		Run: func(ctx *cli.Context, args []string) {
			// TODO: prevent file overwrite if dest file already exits (should this already be an error?)
			// TODO: error when dest directory doesn't exist and args.len > 2
			isdir, err := fs.DirExists(os.DirFS("/"), UnixToFsPath(args[len(args)-1]))
			if CheckErr(ctx, err) {
				return
			}
			if isdir {
				// move all paths into this directory
				dir := AbsPath(args[len(args)-1])
				for _, path := range args[:len(args)-1] {
					src := filepath.Base(path)
					dest := filepath.Join(dir, src)
					err := os.Rename(AbsPath(path), AbsPath(dest))
					if err != nil {
						io.WriteString(ctx, fmt.Sprintln(err))
						continue
					}
				}
			} else {
				err := os.Rename(AbsPath(args[0]), AbsPath(args[1]))
				if CheckErr(ctx, err) {
					return
				}
			}
		},
	}
}

func PwdCmd() *cli.Command {
	return &cli.Command{
		Usage: "pwd",
		Args:  cli.ExactArgs(0),
		Run: func(ctx *cli.Context, args []string) {
			wd, err := os.Getwd()
			if CheckErr(ctx, err) {
				return
			}
			io.WriteString(ctx, fmt.Sprintln(wd))
		},
	}
}

func WriteCmd() *cli.Command {
	return &cli.Command{
		Usage: "write <filepath> [text]...",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			input := append([]byte(strings.Join(args[1:], " ")), '\n')
			err := os.WriteFile(AbsPath(args[0]), input, 0644)
			if CheckErr(ctx, err) {
				return
			}
		},
	}
}

func PrintEnvCmd() *cli.Command {
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

func ExportCmd() *cli.Command {
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
