//go:build js && wasm

package api

import (
	"log"
	"time"

	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

func (s *syscaller) stat(r rpc.Responder, c *rpc.Call) {
	var args []string
	c.Receive(&args)

	fi, err := fs.Stat(s.task.Namespace(), args[0])
	if err != nil {
		r.Return(err)
		return
	}

	r.Return(struct {
		Size    int64
		Mode    uint32
		IsDir   bool
		ModTime time.Time
	}{
		Size:    fi.Size(),
		Mode:    uint32(fi.Mode()),
		IsDir:   fi.IsDir(),
		ModTime: fi.ModTime(),
	})
}

func (s *syscaller) lstat(r rpc.Responder, c *rpc.Call) {
	var args []string
	c.Receive(&args)

	fi, err := fs.Lstat(s.task.Namespace(), args[0])
	if err != nil {
		r.Return(err)
		return
	}

	r.Return(struct {
		Size    int64
		Mode    uint32
		IsDir   bool
		ModTime time.Time
	}{
		Size:    fi.Size(),
		Mode:    uint32(fi.Mode()),
		IsDir:   fi.IsDir(),
		ModTime: fi.ModTime(),
	})
}

func (s *syscaller) fstat(r rpc.Responder, c *rpc.Call) {
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

	fi, err := f.file.Stat()
	if err != nil {
		r.Return(err)
		return
	}

	r.Return(struct {
		Size    int64
		Mode    uint32
		IsDir   bool
		ModTime time.Time
	}{
		Size:    fi.Size(),
		Mode:    uint32(fi.Mode()),
		IsDir:   fi.IsDir(),
		ModTime: fi.ModTime(),
	})
}
