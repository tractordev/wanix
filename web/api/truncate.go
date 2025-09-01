//go:build js && wasm

package api

import (
	"log"

	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) truncate(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	path, ok := args[0].(string)
	if !ok {
		panic("arg 0 is not a string")
	}

	usize, ok := args[1].(uint64)
	if !ok {
		log.Panicf("arg 1 is not a uint64: %T %v", args[1], args[1])
	}
	size := int64(usize)

	err := fs.Truncate(s.task.Namespace(), path, size)
	if err != nil {
		r.Return(err)
		return
	}
}
