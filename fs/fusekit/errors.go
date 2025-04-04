package fusekit

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"syscall"

	"tractor.dev/wanix/fs"
)

func sysErrno(err error) syscall.Errno {
	log.Printf("ERR: %T %v", err, err)
	// printLastFrames()
	switch err {
	case nil:
		return syscall.Errno(0)
	case fs.ErrNotSupported:
		return syscall.EOPNOTSUPP
	case os.ErrPermission:
		return syscall.EPERM
	case os.ErrExist:
		return syscall.EEXIST
	case os.ErrNotExist:
		return syscall.ENOENT
	case os.ErrInvalid:
		return syscall.EINVAL
	}

	switch t := err.(type) {
	case syscall.Errno:
		return t
	case *os.SyscallError:
		return t.Err.(syscall.Errno)
	case *os.PathError:
		return sysErrno(t.Err)
	case *os.LinkError:
		return sysErrno(t.Err)
	default:
		log.Panicf("unsupported error type: %T", err)
		return syscall.EOPNOTSUPP
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
