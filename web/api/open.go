//go:build js && wasm

package api

import (
	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
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
	s.fds[s.fdCounter] = &openFile{file: f, path: args[0]}
	r.Return(s.fdCounter)
}

func (s *syscaller) create(r rpc.Responder, c *rpc.Call) {
	var args []string
	c.Receive(&args)

	f, err := fs.Create(s.task.Namespace(), args[0])
	if err != nil {
		r.Return(err)
		return
	}

	s.fdCounter++
	s.fds[s.fdCounter] = &openFile{file: f, path: args[0]}
	r.Return(s.fdCounter)
}

func (s *syscaller) openFile(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	path, ok := args[0].(string)
	if !ok {
		panic("arg 0 is not a string")
	}

	flags, ok := args[1].(uint64)
	if !ok {
		panic("arg 1 is not a uint64")
	}

	mode, ok := args[2].(uint64)
	if !ok {
		panic("arg 2 is not a uint64")
	}

	f, err := fs.OpenFile(s.task.Namespace(), path, int(flags), fs.FileMode(mode))
	if err != nil {
		r.Return(err)
		return
	}

	s.fdCounter++
	s.fds[s.fdCounter] = &openFile{file: f, path: path}
	r.Return(s.fdCounter)
}
