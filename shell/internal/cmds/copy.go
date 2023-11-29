package cmds

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/wanix/internal/fsutil"
	"tractor.dev/wanix/internal/osfs"
	. "tractor.dev/wanix/shell/internal/sharedutil"
)

// TODO: merge with copyCmd
func CopyCmd2() *cli.Command {
	cmd := &cli.Command{
		Usage: "cp2 SOURCE DEST",
		Args:  cli.MinArgs(2),
		Short: "Recursively copy SOURCE to DEST. DEST must not exist.",
		Run: func(ctx *cli.Context, args []string) {
			srcpath := args[0]
			dstpath := args[1]
			err := fsutil.CopyAll(osfs.New(), AbsPath(srcpath), AbsPath(dstpath))
			if CheckErr(ctx, err) {
				return
			}
		},
	}
	return cmd
}

func CopyCmd() *cli.Command {
	var recursive bool

	cmd := &cli.Command{
		Usage: "cp [-r] <SOURCE DEST | SOURCE... DIRECTORY> ",
		Args:  cli.MinArgs(2),
		Short: "Copy SOURCE to DEST, or multiple SOURCE(s) to DIRECTORY.",
		Run: func(ctx *cli.Context, args []string) {
			// TODO: handle copying directories
			isdir, err := fs.DirExists(os.DirFS("/"), UnixToFsPath(args[len(args)-1]))
			if CheckErr(ctx, err) {
				return
			}
			if isdir {
				// copy all paths to this directory
				dir := AbsPath(args[len(args)-1])

				for _, path := range args[:len(args)-1] {
					srcName := filepath.Base(path)
					dest := filepath.Join(dir, srcName)

					srcIsDir, err := fs.IsDir(os.DirFS("/"), UnixToFsPath(path))
					if CheckErr(ctx, err) {
						continue
					}

					if srcIsDir {
						if !recursive {
							io.WriteString(ctx, fmt.Sprintf("-r not specified; omitting directory '%s'\n", path))
							continue
						}

						err = os.MkdirAll(AbsPath(dest), 0755)
						if CheckErr(ctx, err) {
							continue
						}

						entries, err := os.ReadDir(AbsPath(path))
						if CheckErr(ctx, err) {
							continue
						}

						for _, e := range entries {
							cli.Execute(ctx, CopyCmd(), []string{"-r", filepath.Join(path, e.Name()), dest})
							// commands["cp"](t, fs, []string{"-r", filepath.Join(path, e.Name()), dest})
						}
					} else {
						content, err := os.ReadFile(AbsPath(path))
						if CheckErr(ctx, err) {
							continue
						}
						err = os.WriteFile(AbsPath(dest), content, 0644)
						if CheckErr(ctx, err) {
							continue
						}
					}
				}
			} else {
				content, err := os.ReadFile(AbsPath(args[0]))
				if CheckErr(ctx, err) {
					return
				}

				err = os.WriteFile(AbsPath(args[1]), content, 0644)
				if CheckErr(ctx, err) {
					return
				}
			}
		},
	}

	cmd.Flags().BoolVar(&recursive, "r", false, "Copy recursively")
	return cmd
}
