//go:build js && wasm

package api

import (
	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) remove(r rpc.Responder, c *rpc.Call) {
	var args []string
	c.Receive(&args)

	err := fs.Remove(s.task.Namespace(), args[0])
	if err != nil {
		r.Return(err)
		return
	}
}

func (s *syscaller) removeAll(r rpc.Responder, c *rpc.Call) {
	var args []string
	c.Receive(&args)

	err := fs.RemoveAll(s.task.Namespace(), args[0])
	if err != nil {
		r.Return(err)
		return
	}
}
