package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"log"
	"os"
	"strings"
	"text/template"
)

// Files must be local to the Wanix project root.
var files = []File{
	{Name: "duplex.js", Path: "./kernel/web/lib/duplex.js"},
	{Name: "worker.js", Path: "./kernel/web/lib/worker.js"},
	{Name: "syscall.js", Path: "./kernel/web/lib/syscall.js"},
	{Name: "task.js", Path: "./kernel/web/lib/task.js"},
	{Name: "wasm.js", Path: "./kernel/web/lib/wasm.js"},
	{Name: "host.js", Path: "./kernel/web/lib/host.js"},
	{Name: "indexedfs.js", Path: "./internal/indexedfs/indexedfs.js"},
	{Name: "kernel", Path: "./local/bin/kernel"},
	{Name: "build", Path: "./local/bin/build"},
	{Name: "macro", Path: "./local/bin/micro"},

	// Shell source files
	{Name: "shell/main.go", Path: "shell/main.go"},
	{Name: "shell/copy.go", Path: "shell/copy.go"},
	{Name: "shell/download.go", Path: "shell/download.go"},
	{Name: "shell/main.go", Path: "shell/main.go"},
	{Name: "shell/open.go", Path: "shell/open.go"},
	{Name: "shell/preprocessor.go", Path: "shell/preprocessor.go"},
	{Name: "shell/smallcmds.go", Path: "shell/smallcmds.go"},
	{Name: "shell/tree.go", Path: "shell/tree.go"},
	{Name: "shell/util.go", Path: "shell/util.go"},
	{Name: "shell/watch.go", Path: "shell/watch.go"},
}

type PackMode int

const (
	PackFileData PackMode = iota
	PackFilePaths
)

func PackFilesTo(w io.Writer, mode PackMode) {
	switch mode {
	case PackFileData:
		for i := range files {
			if strings.HasSuffix(files[i].Path, ".js") {
				files[i].Type = "application/javascript"
			} else {
				files[i].Type = "application/octet-stream"
			}

			fi, err := os.Stat(files[i].Path)
			fatal(err)
			files[i].Mtime = fi.ModTime().UnixMilli()

			data, err := os.ReadFile(files[i].Path)
			fatal(err)
			var gzipBuffer bytes.Buffer
			gzipWriter := gzip.NewWriter(&gzipBuffer)
			_, err = gzipWriter.Write(data)
			fatal(err)
			fatal(gzipWriter.Close())
			files[i].Data = base64.StdEncoding.EncodeToString(gzipBuffer.Bytes())
		}

	case PackFilePaths:
		for i := range files {
			files[i].Type = "text/plain"
			files[i].Data = files[i].Path
			fi, err := os.Stat(files[i].Path)
			fatal(err)
			files[i].Mtime = fi.ModTime().UnixMilli()
		}
	}

	t := template.Must(template.New("initdata.tmpl").ParseFiles("./dev/initdata.tmpl"))
	if err := t.Execute(w, files); err != nil {
		log.Fatal(err)
	}
}

type File struct {
	Name  string
	Path  string
	Type  string
	Data  string
	Mtime int64
}

func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
