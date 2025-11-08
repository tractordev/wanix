//go:build js && wasm

package api

import (
	"log"

	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) close(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	fd, ok := args[0].(uint64)
	if !ok {
		log.Panicf("arg 0 is not a uint64: %T %v", args[1], args[1])
	}

	f, ok := s.fds[int(fd)]
	if !ok {
		r.Return(fs.ErrInvalid)
		return
	}

	r.Return(f.file.Close())
	delete(s.fds, int(fd))
}
