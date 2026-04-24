package api

import (
	"log"

	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) write(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	fd, ok := args[0].(uint64)
	if !ok {
		log.Panicf("arg 0 is not a uint64: %T %v", args[1], args[1])
	}

	f, _, err := s.task.FD(int(fd))
	if err != nil {
		r.Return(err)
		return
	}

	data, ok := args[1].([]byte)
	if !ok {
		panic("arg 1 is not a []byte")
	}

	n, err := fs.Write(f, data)
	if err != nil {
		r.Return(err)
		return
	}

	r.Return(uint64(n))
}

func (s *syscaller) writeAt(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	fd, ok := args[0].(uint64)
	if !ok {
		log.Panicf("arg 0 is not a uint64: %T %v", args[1], args[1])
	}

	f, _, err := s.task.FD(int(fd))
	if err != nil {
		r.Return(err)
		return
	}

	data, ok := args[1].([]byte)
	if !ok {
		panic("arg 1 is not a []byte")
	}

	offset, ok := args[2].(uint64)
	if !ok {
		panic("arg 2 is not a uint64")
	}

	n, err := fs.WriteAt(f, data, int64(offset))
	if err != nil {
		r.Return(err)
		return
	}

	r.Return(n)
}
