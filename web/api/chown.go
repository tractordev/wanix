//go:build js && wasm

package api

import (
	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) chown(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	path, ok := args[0].(string)
	if !ok {
		panic("arg 0 is not a string")
	}

	uuid, ok := args[1].(uint64)
	if !ok {
		panic("arg 1 is not a uint64")
	}
	uid := int(uuid)

	ugid, ok := args[2].(uint64)
	if !ok {
		panic("arg 2 is not a uint64")
	}
	gid := int(ugid)

	err := fs.Chown(s.task.Namespace(), path, uid, gid)
	if err != nil {
		r.Return(err)
		return
	}
}

func (s *syscaller) fchown(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	ufd, ok := args[0].(uint64)
	if !ok {
		panic("arg 0 is not a uint64")
	}
	fd := int(ufd)

	uuid, ok := args[1].(uint64)
	if !ok {
		panic("arg 1 is not a uint64")
	}
	uid := int(uuid)

	ugid, ok := args[2].(uint64)
	if !ok {
		panic("arg 2 is not a uint64")
	}
	gid := int(ugid)

	file, ok := s.fds[fd]
	if !ok {
		r.Return(fs.ErrInvalid)
		return
	}

	err := fs.Chown(s.task.Namespace(), file.path, uid, gid)
	if err != nil {
		r.Return(err)
		return
	}
}
