//go:build js && wasm

package api

import (
	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) chmod(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	path, ok := args[0].(string)
	if !ok {
		panic("arg 0 is not a string")
	}

	umode, ok := args[1].(uint64)
	if !ok {
		panic("arg 1 is not a uint64")
	}
	mode := fs.FileMode(umode)

	err := fs.Chmod(s.task.Namespace(), path, mode)
	if err != nil {
		r.Return(err)
		return
	}
}

func (s *syscaller) fchmod(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	ufd, ok := args[0].(uint64)
	if !ok {
		panic("arg 0 is not a uint64")
	}
	fd := int(ufd)

	umode, ok := args[1].(uint64)
	if !ok {
		panic("arg 1 is not a uint64")
	}
	mode := fs.FileMode(umode)

	file, ok := s.fds[fd]
	if !ok {
		r.Return(fs.ErrInvalid)
		return
	}

	err := fs.Chmod(s.task.Namespace(), file.path, mode)
	if err != nil {
		r.Return(err)
		return
	}
}
