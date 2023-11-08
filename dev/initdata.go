package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func PackFilesTo(w io.Writer) {
	var files []File
	for _, path := range []string{
		"./kernel/web/lib/duplex.js",
		"./kernel/web/lib/worker.js",
		"./kernel/web/lib/syscall.js",
		"./kernel/web/lib/task.js",
		"./kernel/web/lib/wasm.js",
		"./kernel/web/lib/host.js",
		"./internal/indexedfs/indexedfs.js",
		"./local/bin/kernel",
		"./local/bin/shell",
	} {
		typ := "application/octet-stream"
		if strings.HasSuffix(path, ".js") {
			typ = "application/javascript"
		}
		data, err := os.ReadFile(path)
		fatal(err)
		var gzipBuffer bytes.Buffer
		gzipWriter := gzip.NewWriter(&gzipBuffer)
		_, err = gzipWriter.Write(data)
		fatal(err)
		fatal(gzipWriter.Close())
		files = append(files, File{
			Name: filepath.Base(path),
			Type: typ,
			Data: base64.StdEncoding.EncodeToString(gzipBuffer.Bytes()),
		})
	}

	t := template.Must(template.New("initdata.tmpl").ParseFiles("./dev/initdata.tmpl"))
	if err := t.Execute(w, files); err != nil {
		log.Fatal(err)
	}
}

type File struct {
	Name string
	Type string
	Data string
}

func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
