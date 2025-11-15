package fusekit

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"syscall"

	"tractor.dev/wanix/fs"
)

func sysErrno(err error) syscall.Errno {
	if err == nil {
		return syscall.Errno(0)
	}

	// Check standard fs errors first
	if errors.Is(err, fs.ErrNotSupported) {
		return syscall.EOPNOTSUPP
	}
	if errors.Is(err, fs.ErrExist) {
		return syscall.EEXIST
	}
	if errors.Is(err, fs.ErrNotExist) {
		return syscall.ENOENT
	}
	if errors.Is(err, fs.ErrInvalid) {
		return syscall.EINVAL
	}
	if errors.Is(err, fs.ErrPermission) {
		return syscall.EPERM
	}
	if errors.Is(err, fs.ErrNotEmpty) {
		return syscall.ENOTEMPTY
	}
	if errors.Is(err, fs.ErrClosed) {
		return syscall.EBADF
	}

	// Check for common I/O errors
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return syscall.ETIMEDOUT
	}
	if errors.Is(err, os.ErrNoDeadline) {
		return syscall.EOPNOTSUPP
	}
	if errors.Is(err, os.ErrClosed) {
		return syscall.EBADF
	}

	// Check specific error types
	switch t := err.(type) {
	case syscall.Errno:
		return t
	case *os.SyscallError:
		if errno, ok := t.Err.(syscall.Errno); ok {
			return errno
		}
		return syscall.EIO
	case *os.PathError:
		return sysErrno(t.Err)
	case *os.LinkError:
		return sysErrno(t.Err)
	default:
		// Log unmapped errors for debugging
		if runtime.GOOS != "js" { // Don't log in WASM builds
			log.Printf("unmapped error: %T %v", err, err)
		}
		return syscall.EIO
	}
}

func printLastFrames() {
	const depth = 3
	var pcs [depth]uintptr
	n := runtime.Callers(2, pcs[:]) // Skip printLastFrames and runtime.Callers itself

	if n == 0 {
		fmt.Println("No callers")
		return
	}

	frames := runtime.CallersFrames(pcs[:n])

	fmt.Println("Recent call stack:")
	for {
		frame, more := frames.Next()
		fmt.Printf("- %s\n    at %s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
}
