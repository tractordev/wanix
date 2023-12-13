package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"syscall/js"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/wanix/internal/jsutil"
)

func downloadCmd() *cli.Command {
	// TODO: compression flag
	return &cli.Command{
		Usage: "dl <path>",
		Args:  cli.ExactArgs(1),
		Short: "Download a Wanix file or directory to the host computer.",
		Run: func(ctx *cli.Context, args []string) {
			target := absPath(args[0])

			fi, err := os.Stat(target)
			if checkErr(ctx, err) {
				return
			}

			var data []byte

			if fi.IsDir() {
				var buf bytes.Buffer
				zw := zip.NewWriter(&buf)

				err := fs.WalkDir(os.DirFS(target), ".", func(path string, d fs.DirEntry, walkErr error) error {
					if walkErr != nil {
						return walkErr
					}
					if path == "." {
						return nil
					}

					name := path
					if d.IsDir() {
						name += "/"
					}

					fw, err := zw.Create(name)
					if err != nil {
						return err
					}

					if d.Type().IsRegular() {
						file, err := os.Open(filepath.Join(target, path))
						if err != nil {
							return err
						}

						if _, err := io.Copy(fw, file); err != nil {
							return err
						}
					}

					return nil
				})

				if checkErr(ctx, err) {
					return
				}
				if err = zw.Close(); checkErr(ctx, err) {
					return
				}

				data = buf.Bytes()
				target = target + ".zip"
			} else {
				data, err = os.ReadFile(target)
				if checkErr(ctx, err) {
					return
				}
			}

			// TODO: there may be a more efficient way of doing this
			// besides passing the file data. Initially we passed a
			// blob but duplex complained about "Iterable/blob should be serialized as iterator".
			// (related: BlobFromFile helper to avoid unecessary operations on indexedfs)

			jsbuf := js.Global().Get("Uint8Array").New(len(data))
			js.CopyBytesToJS(jsbuf, data)
			_, err = jsutil.WanixSyscall("host.download", filepath.Base(target), jsbuf)
			if checkErr(ctx, err) {
				return
			}
		},
	}
}
