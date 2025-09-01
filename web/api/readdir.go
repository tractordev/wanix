//go:build js && wasm

package api

import (
	"log"

	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) readDir(r rpc.Responder, c *rpc.Call) {
	var args []string
	c.Receive(&args)

	// log.Println("ReadDir", args)

	dir, err := fs.ReadDir(s.task.Namespace(), args[0])
	if err != nil {
		log.Println("err:", args[0], err)
		r.Return(err)
		return
	}

	var entries []string
	for _, e := range dir {
		name := e.Name()
		if e.IsDir() {
			name = name + "/"
		}
		entries = append(entries, name)
	}
	r.Return(entries)
}
