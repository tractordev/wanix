//go:build js && wasm

package api

import (
	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) appendFile(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	// log.Println("WriteFile", args)

	name, ok := args[0].(string)
	if !ok {
		panic("arg 0 is not a string")
	}

	data, ok := args[1].([]byte)
	if !ok {
		panic("arg 0 is not a []byte")
	}

	err := fs.AppendFile(s.task.Namespace(), name, data)
	if err != nil {
		r.Return(err)
		return
	}
}
