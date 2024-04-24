package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"path"
	"text/template"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/boot"
)

type File struct {
	Name  string
	Type  string
	Data  string
	Mtime int64
}

// assemble wanix-bootloader from ./boot files
func buildBootloader() ([]byte, error) {
	dir, err := boot.Dir.ReadDir("data")
	if err != nil {
		return nil, err
	}
	var files []File
	for _, d := range dir {
		fi, err := d.Info()
		if err != nil {
			continue
		}

		data, err := fs.ReadFile(boot.Dir, path.Join("data", fi.Name()))
		if err != nil {
			return nil, err
		}
		var gzipBuffer bytes.Buffer
		gzipWriter := gzip.NewWriter(&gzipBuffer)
		_, err = gzipWriter.Write(data)
		if err != nil {
			return nil, err
		}
		if err := gzipWriter.Close(); err != nil {
			return nil, err
		}

		files = append(files, File{
			Name:  fi.Name(),
			Type:  "text/javascript", // todo: fix when needed
			Mtime: fi.ModTime().UnixMilli(),
			Data:  base64.StdEncoding.EncodeToString(gzipBuffer.Bytes()),
		})
	}

	loader, err := fs.ReadFile(boot.Dir, "loader.js")
	if err != nil {
		return nil, err
	}

	t := template.Must(template.New("bootloader.tmpl").ParseFS(boot.Dir, "bootloader.tmpl"))
	var buf bytes.Buffer
	if err := t.Execute(&buf, map[string]any{
		"Loader": string(loader),
		"Files":  files,
	}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func bootfilesCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "bootfiles",
		Short: "write out wanix boot files",
		Run: func(ctx *cli.Context, args []string) {
			bl, err := buildBootloader()
			fatal(err)
			fatal(os.WriteFile("wanix-bootloader.js", bl, 0644))
			fmt.Println("Wrote file wanix-bootloader.js")

			kernel, err := fs.ReadFile(boot.Dir, "kernel.gz")
			fatal(err)
			fatal(os.WriteFile("wanix-kernel.gz", kernel, 0644))
			fmt.Println("Wrote file wanix-kernel.gz")

			initfs, err := fs.ReadFile(boot.Dir, "initfs.gz")
			fatal(err)
			fatal(os.WriteFile("wanix-initfs.gz", initfs, 0644))
			fmt.Println("Wrote file wanix-initfs.gz")
		},
	}
	return cmd
}
