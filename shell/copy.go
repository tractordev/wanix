package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/wanix/internal/fsutil"
	"tractor.dev/wanix/internal/osfs"
)

func copyCmd() *cli.Command {
	var recursive bool
	var overwrite bool

	cmd := &cli.Command{
		Usage: "cp [-r] [-overwrite] <SOURCE DEST | SOURCE... DIRECTORY> ",
		Args:  cli.MinArgs(2),
		Short: "Copy SOURCE to DEST, or SOURCE(s) to DIRECTORY.",
		Long: `
Copies a single SOURCE to DEST. If DEST exists and is a directory, copy SOURCE to DEST/SOURCE_NAME.
Can also copy multiple SOURCEs to a DIRECTORY, under DIRECTORY/SOURCE_NAME.
Errors if final DEST already exists and "-overwrite" flag isn't specified.
Omits any SOURCE that is a directory if "-r" flag isn't specified.
`,
		Run: func(ctx *cli.Context, args []string) {
			DEST := absPath(args[len(args)-1])
			dstInfo, dstErr := os.Stat(DEST)
			if dstErr != nil && !os.IsNotExist(dstErr) {
				io.WriteString(ctx, fmt.Sprintln(dstErr))
				return
			}

			DEST_exists := dstErr == nil
			DEST_isDir := DEST_exists && dstInfo.IsDir()

			if len(args)-1 >= 2 && !DEST_isDir {
				io.WriteString(ctx, fmt.Sprintf("target '%s' is not a directory\n", args[len(args)-1]))
				return
			}

			for _, SOURCE := range args[:len(args)-1] {
				SOURCE = absPath(SOURCE)
				srcInfo, err := os.Stat(SOURCE)
				if checkErr(ctx, err) {
					continue
				}

				if srcInfo.IsDir() && !recursive {
					io.WriteString(ctx, fmt.Sprintf("'-r' not specified; omitting directory '%s'\n", SOURCE))
					continue
				}

				var finalDest string
				var finalDestExists bool
				if DEST_isDir {
					finalDest = filepath.Join(DEST, filepath.Base(SOURCE))
					finalDestExists, err = fs.Exists(osfs.New(), unixToFsPath(finalDest))
					if checkErr(ctx, err) {
						continue
					}
				} else {
					finalDest = DEST
					finalDestExists = DEST_exists
				}

				if srcInfo.IsDir() && strings.HasPrefix(finalDest, SOURCE) {
					io.WriteString(ctx, fmt.Sprintf("cannot copy directory '%s' into itself, '%s'; omitting '%[0]s'\n", SOURCE, finalDest))
					continue
				}

				if finalDest == SOURCE {
					io.WriteString(ctx, fmt.Sprintf("'%[0]s' and '%[0]s' are the same file; omitting '%[0]s'\n", SOURCE))
					continue
				}

				// fmt.Printf("recursive: %v, overwrite: %v, SOURCE: %s, DEST: %s, DEST_exists: %v, DEST_isDir: %v finalDest: %s, finalDestExists: %v,\n",
				// 	recursive, overwrite, SOURCE, DEST, DEST_exists, DEST_isDir, finalDest, finalDestExists,
				// )

				if finalDestExists {
					if !overwrite {
						io.WriteString(ctx, fmt.Sprintf("'%s' exists and '-overwrite' is not specified; omitting '%s'\n", finalDest, SOURCE))
						continue
					}

					if err := os.RemoveAll(finalDest); checkErr(ctx, err) {
						return
					}
				}

				if err := fsutil.CopyAll(osfs.New(), unixToFsPath(SOURCE), unixToFsPath(finalDest)); checkErr(ctx, err) {
					return
				}
			}
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&recursive, "r", false, "Copy file/directory(s) recursively.")
	flags.BoolVar(&overwrite, "overwrite", false, "Overwrite any existing file/directory(s) at the destination.")
	return cmd
}
