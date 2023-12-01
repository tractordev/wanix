package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/wanix/internal/fsutil"
	"tractor.dev/wanix/internal/osfs"
)

// TODO: merge with copyCmd
func copyCmd2() *cli.Command {
	cmd := &cli.Command{
		Usage: "cp2 SOURCE DEST",
		Args:  cli.MinArgs(2),
		Short: "Recursively copy SOURCE to DEST. DEST must not exist.",
		Run: func(ctx *cli.Context, args []string) {
			srcpath := args[0]
			dstpath := args[1]
			err := fsutil.CopyAll(osfs.New(), absPath(srcpath), absPath(dstpath))
			if checkErr(ctx, err) {
				return
			}
		},
	}
	return cmd
}

func copyCmd() *cli.Command {
	var recursive bool

	cmd := &cli.Command{
		Usage: "cp [-r] <SOURCE DEST | SOURCE... DIRECTORY> ",
		Args:  cli.MinArgs(2),
		Short: "Copy SOURCE to DEST, or multiple SOURCE(s) to DIRECTORY.",
		Run: func(ctx *cli.Context, args []string) {
			// TODO: handle copying directories
			isdir, err := fs.DirExists(os.DirFS("/"), unixToFsPath(args[len(args)-1]))
			if checkErr(ctx, err) {
				return
			}
			if isdir {
				// copy all paths to this directory
				dir := absPath(args[len(args)-1])

				for _, path := range args[:len(args)-1] {
					srcName := filepath.Base(path)
					dest := filepath.Join(dir, srcName)

					srcIsDir, err := fs.IsDir(os.DirFS("/"), unixToFsPath(path))
					if checkErr(ctx, err) {
						continue
					}

					if srcIsDir {
						if !recursive {
							io.WriteString(ctx, fmt.Sprintf("-r not specified; omitting directory '%s'\n", path))
							continue
						}

						err = os.MkdirAll(absPath(dest), 0755)
						if checkErr(ctx, err) {
							continue
						}

						entries, err := os.ReadDir(absPath(path))
						if checkErr(ctx, err) {
							continue
						}

						for _, e := range entries {
							cli.Execute(ctx, copyCmd(), []string{"-r", filepath.Join(path, e.Name()), dest})
							// commands["cp"](t, fs, []string{"-r", filepath.Join(path, e.Name()), dest})
						}
					} else {
						content, err := os.ReadFile(absPath(path))
						if checkErr(ctx, err) {
							continue
						}
						err = os.WriteFile(absPath(dest), content, 0644)
						if checkErr(ctx, err) {
							continue
						}
					}
				}
			} else {
				content, err := os.ReadFile(absPath(args[0]))
				if checkErr(ctx, err) {
					return
				}

				err = os.WriteFile(absPath(args[1]), content, 0644)
				if checkErr(ctx, err) {
					return
				}
			}
		},
	}

	cmd.Flags().BoolVar(&recursive, "r", false, "Copy recursively")
	return cmd
}
