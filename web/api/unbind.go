//go:build js && wasm

package api

import (
	"tractor.dev/toolkit-go/duplex/rpc"
)

func (s *syscaller) unbind(r rpc.Responder, c *rpc.Call) {
	var args []string
	c.Receive(&args)

	ns := s.task.Namespace()
	err := ns.Unbind(ns, args[0], args[1])
	if err != nil {
		r.Return(err)
		return
	}
}
