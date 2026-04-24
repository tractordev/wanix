package api

import (
	"log"

	"tractor.dev/toolkit-go/duplex/rpc"
)

func (s *syscaller) close(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	fd, ok := args[0].(uint64)
	if !ok {
		log.Panicf("arg 0 is not a uint64: %T %v", args[1], args[1])
	}

	r.Return(s.task.CloseFD(int(fd)))
}
