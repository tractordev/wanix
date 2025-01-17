package fusekit

import (
	"context"
	"io"
	"log"
	"syscall"

	iofs "tractor.dev/wanix/fs"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type handle struct {
	file iofs.File
	path string
}

var _ = (fs.FileReader)((*handle)(nil))

func (h *handle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	log.Println("read", h.path, off)

	n, err := iofs.ReadAt(h.file, dest, off)
	if err != nil && err != io.EOF {
		return nil, sysErrno(err)
	}

	return fuse.ReadResultData(dest[:n]), 0
}

var _ = (fs.FileWriter)((*handle)(nil))

func (h *handle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	log.Println("write", h.path, off)

	n, err := iofs.WriteAt(h.file, data, off)
	if err != nil {
		return 0, sysErrno(err)
	}

	return uint32(n), 0
}

var _ = (fs.FileFlusher)((*handle)(nil))

func (h *handle) Flush(ctx context.Context) syscall.Errno {
	log.Println("flush", h.path)

	if err := h.file.Close(); err != nil {
		return sysErrno(err)
	}

	return 0
}

var _ = (fs.FileFsyncer)((*handle)(nil))

func (h *handle) Fsync(ctx context.Context, flags uint32) syscall.Errno {
	log.Println("fsync", h.path)
	return 0
}
