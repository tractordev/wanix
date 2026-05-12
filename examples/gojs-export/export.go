//go:build js && wasm

package main

import (
	"context"
	"io/fs"
	"log"
	"time"

	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/gojs"
)

func main() {
	fsys := fskit.MapFS{
		"myfile": fskit.RawNode([]byte("hello, world\n")),
		"time": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			return &fskit.FuncFile{
				Node: fskit.Entry(name, 0555),
				ReadFunc: func(n *fskit.Node) error {
					fskit.SetData(n, []byte(time.Now().Format(time.RFC3339)+"\n"))
					return nil
				},
			}, nil
		}),
	}
	if err := gojs.Export(fsys, false); err != nil {
		log.Fatal(err)
	}
	select {}
}
