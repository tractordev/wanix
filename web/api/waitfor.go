//go:build js && wasm

package api

import (
	"log"
	"time"

	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) waitFor(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	path, ok := args[0].(string)
	if !ok {
		panic("arg 0 is not a string")
	}

	timeout, ok := args[1].(uint64)
	if !ok {
		log.Panicf("arg 1 is not a uint64: %T %v", args[1], args[1])
	}

	deadline := time.Now().Add(time.Duration(timeout) * time.Millisecond)
	for time.Now().Before(deadline) {
		exists, err := fs.Exists(s.task.Namespace(), path)
		if err != nil {
			r.Return(err)
			return
		}
		if exists {
			r.Return()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	r.Return(fs.ErrNotExist)
}
