//go:build js && wasm

package api

import (
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
