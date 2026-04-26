package api

import (
	"log"
	"time"

	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/pstat"
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
		Size: fi.Size(),
		Mode: pstat.FileModeToUnixMode(fi.Mode()),
		// Mode:    uint32(fi.Mode()),
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
		Size: fi.Size(),
		Mode: pstat.FileModeToUnixMode(fi.Mode()),
		// Mode:    uint32(fi.Mode()),
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

	f, _, err := s.task.FD(int(fd))
	if err != nil {
		log.Println("fstat error", err, fd)
		r.Return(err)
		return
	}

	fi, err := f.Stat()
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
		Size: fi.Size(),
		Mode: pstat.FileModeToUnixMode(fi.Mode()),
		// Mode:    uint32(fi.Mode()),
		IsDir:   fi.IsDir(),
		ModTime: fi.ModTime(),
	})
}
