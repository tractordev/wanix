//go:build js && wasm

package api

import (
	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) rename(r rpc.Responder, c *rpc.Call) {
	var args []string
	c.Receive(&args)

	err := fs.Rename(s.task.Namespace(), args[0], args[1])
	if err != nil {
		r.Return(err)
		return
	}
}
