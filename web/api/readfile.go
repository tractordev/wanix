//go:build js && wasm

package api

import (
	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) readFile(r rpc.Responder, c *rpc.Call) {
	var args []string
	c.Receive(&args)

	// log.Println("ReadFile", args)

	b, err := fs.ReadFile(s.task.Namespace(), args[0])
	if err != nil {
		r.Return(err)
		return
	}

	r.Return(b)
}
