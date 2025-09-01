//go:build js && wasm

package api

import (
	"tractor.dev/toolkit-go/duplex/rpc"
)

func (s *syscaller) open(r rpc.Responder, c *rpc.Call) {
	var args []string
	c.Receive(&args)

	f, err := s.task.Namespace().Open(args[0])
	if err != nil {
		r.Return(err)
		return
	}

	s.fdCounter++
	s.fds[s.fdCounter] = f
	r.Return(s.fdCounter)
}
