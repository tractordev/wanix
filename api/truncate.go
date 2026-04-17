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

func (s *syscaller) ftruncate(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	ufd, ok := args[0].(uint64)
	if !ok {
		panic("arg 0 is not a uint64")
	}
	fd := int(ufd)

	usize, ok := args[1].(uint64)
	if !ok {
		panic("arg 1 is not a uint64")
	}
	size := int64(usize)

	file, ok := s.fds[fd]
	if !ok {
		r.Return(fs.ErrInvalid)
		return
	}

	err := fs.Truncate(s.task.Namespace(), file.path, size)
	if err != nil {
		r.Return(err)
		return
	}
}
