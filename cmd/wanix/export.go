//go:build !js && !wasm

package main

import (
	"archive/tar"
	"bytes"
	"io"
	"log"
	"os"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/external/linux"
	v86 "tractor.dev/wanix/external/v86"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/shell"
	"tractor.dev/wanix/wasm/assets"
)

func exportCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "export",
		Short: "",
		Run: func(ctx *cli.Context, args []string) {
			log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)

			fsys := fskit.UnionFS{assets.Dir, fskit.MapFS{
				"v86":             v86.Dir,
				"linux":           linux.Dir,
				"shell":           shell.Dir,
				"wanix.bundle.js": fskit.RawNode(fs.FileMode(0600), assets.WanixBundle()),
			}}

			// Create a new tar writer
			var buf bytes.Buffer
			tarWriter := tar.NewWriter(&buf)

			fatal(addFileToTar(tarWriter, fsys, "wanix.bundle.js"))
			fatal(addFileToTar(tarWriter, fsys, "wanix-sw.js"))
			fatal(addFileToTar(tarWriter, fsys, "wanix.wasm"))
			fatal(addFileToTar(tarWriter, fsys, "wanix.css"))
			fatal(addFileToTar(tarWriter, fsys, "favicon.ico"))
			fatal(addFileToTar(tarWriter, fsys, "shell/shell.tgz"))
			fatal(addFileToTar(tarWriter, fsys, "linux/bzImage"))
			fatal(addFileToTar(tarWriter, fsys, "v86/v86.wasm"))
			fatal(addFileToTar(tarWriter, fsys, "v86/seabios.bin"))
			fatal(addFileToTar(tarWriter, fsys, "v86/vgabios.bin"))
			fatal(addFileToTar(tarWriter, fsys, "index.html"))

			tarWriter.Close()
			os.Stdout.Write(buf.Bytes())
		},
	}
	return cmd
}

func addFileToTar(tarWriter *tar.Writer, fsys fs.FS, name string) error {
	// Open the file
	file, err := fsys.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get file information for header
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	// Create a tar header from the file info
	header, err := tar.FileInfoHeader(fileInfo, "")
	if err != nil {
		return err
	}

	header.Name = name

	// Write the header to the tar archive
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	// Copy the file contents to the tar writer
	_, err = io.Copy(tarWriter, file)
	return err
}
