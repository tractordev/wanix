package fusekit

import (
	"context"
	"errors"
	"io"
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
	// log.Println("read", h.path, off)

	n, err := iofs.ReadAt(h.file, dest, off)
	if err != nil && err != io.EOF {
		return nil, sysErrno(err)
	}

	return fuse.ReadResultData(dest[:n]), 0
}

var _ = (fs.FileWriter)((*handle)(nil))

func (h *handle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	// log.Println("write", h.path, off)

	n, err := iofs.WriteAt(h.file, data, off)
	if err != nil {
		return 0, sysErrno(err)
	}

	return uint32(n), 0
}

var _ = (fs.FileFlusher)((*handle)(nil))

func (h *handle) Flush(ctx context.Context) syscall.Errno {
	// log.Println("flush", h.path)

	if err := h.file.Close(); err != nil {
		return sysErrno(err)
	}

	return 0
}

var _ = (fs.FileFsyncer)((*handle)(nil))

func (h *handle) Fsync(ctx context.Context, flags uint32) syscall.Errno {
	// log.Println("fsync", h.path)

	// Try to sync if the file supports it
	if err := iofs.Sync(h.file); err != nil {
		// Ignore "not supported" errors gracefully
		if !errors.Is(err, iofs.ErrNotSupported) {
			return sysErrno(err)
		}
	}

	return 0
}

var _ = (fs.FileLseeker)((*handle)(nil))

func (h *handle) Lseek(ctx context.Context, off uint64, whence uint32) (uint64, syscall.Errno) {
	// log.Println("lseek", h.path, off, whence)

	newOff, err := iofs.Seek(h.file, int64(off), int(whence))
	if err != nil {
		return 0, sysErrno(err)
	}

	return uint64(newOff), 0
}

var _ = (fs.FileSetattrer)((*handle)(nil))

func (h *handle) Setattr(ctx context.Context, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	// log.Println("file setattr", h.path, in.Valid)

	// Try to handle size via Truncate on the file
	if in.Valid&fuse.FATTR_SIZE != 0 {
		// Check if file has Truncate method
		if tf, ok := h.file.(interface{ Truncate(int64) error }); ok {
			if err := tf.Truncate(int64(in.Size)); err != nil {
				return sysErrno(err)
			}
		}
	}

	// For other attributes, we would need the filesystem reference
	// This is a limitation of operating via file handle
	// Most other attributes would be handled by NodeSetattrer instead

	// Get updated file info
	fi, err := h.file.Stat()
	if err != nil {
		return sysErrno(err)
	}
	applyStat(&out.Attr, fi)

	return 0
}

var _ = (fs.FileAllocater)((*handle)(nil))

func (h *handle) Allocate(ctx context.Context, off uint64, size uint64, mode uint32) syscall.Errno {
	// log.Println("allocate", h.path, off, size, mode)

	// Check if file supports Allocate
	if af, ok := h.file.(interface {
		Allocate(mode uint32, off int64, size int64) error
	}); ok {
		if err := af.Allocate(mode, int64(off), int64(size)); err != nil {
			return sysErrno(err)
		}
		return 0
	}

	// Not supported by this file
	return syscall.EOPNOTSUPP
}

var _ = (fs.FileGetlker)((*handle)(nil))

func (h *handle) Getlk(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32, out *fuse.FileLock) syscall.Errno {
	// log.Println("getlk", h.path)

	// File locking is not supported
	// Return EOPNOTSUPP to indicate locks are not available
	return syscall.EOPNOTSUPP
}

var _ = (fs.FileSetlker)((*handle)(nil))

func (h *handle) Setlk(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	// log.Println("setlk", h.path)

	// File locking is not supported
	return syscall.EOPNOTSUPP
}

var _ = (fs.FileSetlkwer)((*handle)(nil))

func (h *handle) Setlkw(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32) syscall.Errno {
	// log.Println("setlkw", h.path)

	// File locking is not supported
	return syscall.EOPNOTSUPP
}

var _ = (fs.FileGetattrer)((*handle)(nil))

func (h *handle) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	// log.Println("file getattr", h.path)

	fi, err := h.file.Stat()
	if err != nil {
		return sysErrno(err)
	}
	applyStat(&out.Attr, fi)

	return 0
}
