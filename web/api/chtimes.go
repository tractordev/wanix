//go:build js && wasm

package api

import (
	"time"

	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) chtimes(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	path, ok := args[0].(string)
	if !ok {
		panic("arg 0 is not a string")
	}

	// atime and mtime are in seconds (with fractional parts)
	atimeSec, ok := args[1].(float64)
	if !ok {
		panic("arg 1 is not a float64")
	}
	atime := time.Unix(int64(atimeSec), int64((atimeSec-float64(int64(atimeSec)))*1e9))

	mtimeSec, ok := args[2].(float64)
	if !ok {
		panic("arg 2 is not a float64")
	}
	mtime := time.Unix(int64(mtimeSec), int64((mtimeSec-float64(int64(mtimeSec)))*1e9))

	err := fs.Chtimes(s.task.Namespace(), path, atime, mtime)
	if err != nil {
		r.Return(err)
		return
	}
}
