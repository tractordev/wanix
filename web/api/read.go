//go:build js && wasm

package api

import (
	"io"
	"log"

	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) read(r rpc.Responder, c *rpc.Call) {
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

	count, ok := args[1].(uint64)
	if !ok {
		panic("arg 1 is not a uint64")
	}

	buf := make([]byte, count)
	n, err := f.Read(buf)
	if err == io.EOF {
		r.Return(nil)
		return
	}
	if err != nil {
		r.Return(err)
		return
	}

	r.Return(buf[:n])
}
