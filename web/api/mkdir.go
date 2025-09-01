//go:build js && wasm

package api

import (
	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) mkdir(r rpc.Responder, c *rpc.Call) {
	var args []string
	c.Receive(&args)

	err := fs.Mkdir(s.task.Namespace(), args[0], 0755)
	if err != nil {
		r.Return(err)
		return
	}
}

func (s *syscaller) mkdirAll(r rpc.Responder, c *rpc.Call) {
	var args []string
	c.Receive(&args)

	err := fs.MkdirAll(s.task.Namespace(), args[0], 0755)
	if err != nil {
		r.Return(err)
		return
	}
}
