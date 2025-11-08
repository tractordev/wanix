//go:build js && wasm

package api

import (
	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) readlink(r rpc.Responder, c *rpc.Call) {
	var args []string
	c.Receive(&args)

	target, err := fs.Readlink(s.task.Namespace(), args[0])
	if err != nil {
		r.Return(err)
		return
	}

	r.Return(target)
}

