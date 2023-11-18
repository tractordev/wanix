package fs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"syscall/js"
	"time"

	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/wanix/internal/httpfs"
	"tractor.dev/wanix/internal/indexedfs"
	"tractor.dev/wanix/internal/jsutil"

	"tractor.dev/wanix/internal/mountablefs"
)

func log(args ...any) {
	js.Global().Get("console").Call("log", args...)
}

type Service struct {
	fsys *mountablefs.FS

	fds    map[int]*fd
	nextFd int

	mu sync.Mutex
}

type fd struct {
	fs.File
	Path string
}

func (s *Service) Initialize() {
	s.fds = make(map[int]*fd)
	s.nextFd = 1000

	ifs, err := indexedfs.New()
	if err != nil {
		panic(err)
	}
	s.fsys = mountablefs.New(ifs)

	// setup dirs
	fs.MkdirAll(s.fsys, "app", 0755)
	fs.MkdirAll(s.fsys, "cmd", 0755)
	fs.MkdirAll(s.fsys, "sys", 0755)

	devURL := fmt.Sprintf("%ssys/dev", js.Global().Get("hostURL").String())
	resp, err := http.DefaultClient.Get(devURL)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode == 200 {
		if err := s.fsys.Mount(httpfs.New(devURL), "/sys/dev"); err != nil {
			panic(err)
		}
	}
}

func (s *Service) InitializeJS() {
	fsObj := js.Global().Get("fs")
	fsObj.Set("write", js.FuncOf(s.write))
	fsObj.Set("chmod", js.FuncOf(s.chmod))
	fsObj.Set("fchmod", js.FuncOf(s.fchmod))
	fsObj.Set("chown", js.FuncOf(s.chown))
	fsObj.Set("lchown", js.FuncOf(s.lchown))
	fsObj.Set("fchown", js.FuncOf(s.fchown))
	fsObj.Set("close", js.FuncOf(s.close))
	fsObj.Set("fstat", js.FuncOf(s.fstat))
	fsObj.Set("lstat", js.FuncOf(s.lstat))
	fsObj.Set("stat", js.FuncOf(s.stat))
	fsObj.Set("mkdir", js.FuncOf(s.mkdir))
	fsObj.Set("open", js.FuncOf(s.open))
	fsObj.Set("read", js.FuncOf(s.read))
	fsObj.Set("readdir", js.FuncOf(s.readdir))
	fsObj.Set("rename", js.FuncOf(s.rename))
	fsObj.Set("rmdir", js.FuncOf(s.rmdir))
	fsObj.Set("unlink", js.FuncOf(s.unlink))
	fsObj.Set("fsync", js.FuncOf(s.fsync))
	fsObj.Set("utimes", js.FuncOf(s.utimes))

	// TODO later
	// ftruncate(fd, length, callback) { callback(enosys()); },
	// link(path, link, callback) { callback(enosys()); },
	// readlink(path, callback) { callback(enosys()); },
	// symlink(path, link, callback) { callback(enosys()); },
	// truncate(path, length, callback) { callback(enosys()); },

}

func cleanPath(path string) string {
	p := strings.TrimLeft(filepath.Clean(path), "/")
	if p == "" || p == "/" {
		return "."
	}
	return p
}

// open(path, flags, mode, callback)
func (s *Service) open(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	flag := args[1].Int()
	mode := args[2].Int()
	cb := args[3]
	var flags []string
	if flag&os.O_CREATE != 0 {
		flags = append(flags, "create")
	}
	if flag&os.O_APPEND != 0 {
		flags = append(flags, "append")
	}
	if flag&os.O_EXCL != 0 {
		flags = append(flags, "excl")
	}
	if flag&os.O_RDONLY != 0 {
		flags = append(flags, "rdonly")
	}
	if flag&os.O_RDWR != 0 {
		flags = append(flags, "rw")
	}
	if flag&os.O_SYNC != 0 {
		flags = append(flags, "sync")
	}
	if flag&os.O_TRUNC != 0 {
		flags = append(flags, "trunc")
	}
	if flag&os.O_WRONLY != 0 {
		flags = append(flags, "wronly")
	}

	go func() {
		log("open", path, s.nextFd, strings.Join(flags, ","), fmt.Sprintf("%o\n", mode))

		f, err := s.fsys.OpenFile(path, flag, fs.FileMode(mode))
		if err != nil {
			if f != nil {
				log("opened")
			}
			cb.Invoke(jsutil.ToJSError(err))
			return
		}

		s.mu.Lock()
		fdi := s.nextFd
		s.fds[fdi] = &fd{
			File: f,
			Path: path,
		}
		s.nextFd++
		s.mu.Unlock()

		cb.Invoke(nil, fdi)
	}()

	return nil
}

// read(fd, buffer, offset, length, position, callback)
func (s *Service) read(this js.Value, args []js.Value) any {
	fd := args[0].Int()
	jsbuf := args[1]
	// offset := args[2] unused
	length := args[3].Int()
	pos := args[4]
	cb := args[5]
	log("read", fd)

	if fd == 0 {
		go js.Global().Get("stdin").Invoke(jsbuf, cb)

		// if working with stdin, we dont want to block the main thread
		// that would write to stdin from javascript
		// go func() {
		// 	buf := make([]byte, length)
		// 	n, err := StdinBuf.Read(buf)
		// 	if err != nil {
		// 		cb.Invoke(jsutil.ToJSError(err), 0)
		// 		return
		// 	}
		// 	js.CopyBytesToJS(jsbuf, buf[:n])
		// 	cb.Invoke(nil, n)
		// }()

		return nil
	}

	go func() {
		s.mu.Lock()
		f, ok := s.fds[fd]
		s.mu.Unlock()
		if !ok {
			cb.Invoke(jsutil.ToJSError(syscall.EBADF), 0)
			return
		}

		if rs, ok := f.File.(io.ReadSeeker); ok && !pos.IsNull() {
			_, err := rs.Seek(int64(pos.Int()), 0)
			if err != nil {
				cb.Invoke(jsutil.ToJSError(err), 0)
				return
			}
		}

		buf := make([]byte, length)
		n, err := f.Read(buf)
		if n > 0 {
			js.CopyBytesToJS(jsbuf, buf[:n])
		}
		if err != nil && err != io.EOF {
			cb.Invoke(jsutil.ToJSError(err), n)
			return
		}

		cb.Invoke(nil, n)
	}()

	return nil
}

// write(fd, buf, offset, length, position, callback)
func (s *Service) write(this js.Value, args []js.Value) any {
	fd := args[0].Int()
	jsbuf := args[1]
	// offset := args[2].Int() unused
	length := args[3].Int()
	pos := args[4]
	cb := args[5]

	go func() {
		log("write", fd)

		if fd == 1 {
			js.Global().Get("stdout").Invoke(jsbuf)
			cb.Invoke(nil, length)
			return
		}
		if fd == 2 {
			js.Global().Get("stderr").Invoke(jsbuf)
			cb.Invoke(nil, length)
			return
		}

		s.mu.Lock()
		f, ok := s.fds[fd]
		s.mu.Unlock()
		if !ok {
			cb.Invoke(jsutil.ToJSError(syscall.EBADF))
			return
		}

		if ws, ok := f.File.(io.WriteSeeker); ok && !pos.IsNull() {
			_, err := ws.Seek(int64(pos.Int()), 0)
			if err != nil {
				cb.Invoke(jsutil.ToJSError(err))
				return
			}
		}

		buf := make([]byte, length)
		js.CopyBytesToGo(buf, jsbuf)

		if fw, ok := f.File.(io.Writer); ok {
			n, err := fw.Write(buf)
			if err != nil {
				cb.Invoke(jsutil.ToJSError(err))
				return
			}
			cb.Invoke(nil, n)
		}
	}()

	return nil
}

// readdir(path, callback)
func (s *Service) readdir(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	cb := args[1]

	go func() {
		log("readdir", path)

		fi, err := fs.ReadDir(s.fsys, path)
		if err != nil {
			cb.Invoke(jsutil.ToJSError(err))
			return
		}

		names := []any{}
		for _, info := range fi {
			names = append(names, info.Name())
		}

		cb.Invoke(nil, names)
	}()

	return nil
}

func (s *Service) _stat(path string, cb js.Value) {
	fi, err := s.fsys.Stat(path)
	if err != nil {
		cb.Invoke(jsutil.ToJSError(err))
		return
	}
	m := uint32(fi.Mode())
	if fi.IsDir() {
		// we're building syscall.Stat_t which uses
		// a different mask for IsDir than just ModeDir
		m |= syscall.S_IFDIR
	}
	// log("stat", fi.Name(), fi.IsDir(), uint32(fi.Mode()), fi.Size())
	cb.Invoke(nil, map[string]any{
		"mode":    m,
		"dev":     0,
		"ino":     0,
		"nlink":   0,
		"uid":     0,
		"gid":     0,
		"rdev":    0,
		"size":    fi.Size(),
		"blksize": 0,
		"blocks":  0,
		"atimeMs": 0, // not supported by memmap fs
		"mtimeMs": fi.ModTime().UnixMilli(),
		"ctimeMs": 0, // not supported by memmap fs
		"isDirectory": js.FuncOf(func(this js.Value, args []js.Value) any {
			return fi.IsDir()
		}),
	})
}

// stat(path, callback)
func (s *Service) stat(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	cb := args[1]

	go func() {
		log("stat", path)
		s._stat(path, cb)
	}()

	return nil
}

// lstat(path, callback)
func (s *Service) lstat(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	cb := args[1]

	go func() {
		log("lstat", path)
		s._stat(path, cb)
	}()

	return nil
}

// fstat(fd, callback)
func (s *Service) fstat(this js.Value, args []js.Value) any {
	fd := args[0].Int()
	cb := args[1]

	go func() {
		log("fstat", fd)

		s.mu.Lock()
		f, ok := s.fds[fd]
		s.mu.Unlock()
		if !ok {
			cb.Invoke(jsutil.ToJSError(syscall.EBADF))
			return
		}

		s._stat(f.Path, cb)
	}()

	return nil
}

// close(fd, callback)
func (s *Service) close(this js.Value, args []js.Value) any {
	fd := args[0].Int()
	cb := args[1]

	go func() {
		log("close", fd)

		s.mu.Lock()
		f, ok := s.fds[fd]
		s.mu.Unlock()
		if !ok {
			cb.Invoke(jsutil.ToJSError(syscall.EBADF))
			return
		}

		if err := f.Close(); err != nil {
			cb.Invoke(jsutil.ToJSError(err))
			return
		}

		s.mu.Lock()
		delete(s.fds, fd)
		s.mu.Unlock()

		cb.Invoke(nil)
	}()

	return nil
}

// chown(path, uid, gid, callback)
func (s *Service) chown(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	uid := args[1].Int()
	gid := args[2].Int()
	cb := args[3]

	go func() {
		log("chown", path)

		if err := s.fsys.Chown(path, uid, gid); err != nil {
			cb.Invoke(jsutil.ToJSError(err))
			return
		}

		cb.Invoke(nil)
	}()

	return nil
}

// fchown(fd, uid, gid, callback)
func (s *Service) fchown(this js.Value, args []js.Value) any {
	fd := args[0].Int()
	uid := args[1].Int()
	gid := args[2].Int()
	cb := args[3]

	go func() {
		log("fchown", fd)

		s.mu.Lock()
		f, ok := s.fds[fd]
		s.mu.Unlock()
		if !ok {
			cb.Invoke(jsutil.ToJSError(syscall.EBADF))
			return
		}

		if err := s.fsys.Chown(f.Path, uid, gid); err != nil {
			cb.Invoke(jsutil.ToJSError(err))
			return
		}

		cb.Invoke(nil)
	}()

	return nil
}

// lchown(path, uid, gid, callback)
func (s *Service) lchown(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	uid := args[1].Int()
	gid := args[2].Int()
	cb := args[3]

	go func() {
		log("lchown", path)

		if err := s.fsys.Chown(path, uid, gid); err != nil {
			cb.Invoke(jsutil.ToJSError(err))
			return
		}

		cb.Invoke(nil)
	}()

	return nil
}

// chmod(path, mode, callback)
func (s *Service) chmod(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	mode := args[1].Int()
	cb := args[2]

	go func() {
		log("chmod", path)

		if err := s.fsys.Chmod(path, fs.FileMode(mode)); err != nil {
			cb.Invoke(jsutil.ToJSError(err))
			return
		}

		cb.Invoke(nil)
	}()

	return nil
}

// fchmod(fd, mode, callback)
func (s *Service) fchmod(this js.Value, args []js.Value) any {
	fd := args[0].Int()
	mode := args[1].Int()
	cb := args[2]

	go func() {
		log("fchmod", fd)

		s.mu.Lock()
		f, ok := s.fds[fd]
		s.mu.Unlock()
		if !ok {
			cb.Invoke(jsutil.ToJSError(syscall.EBADF))
			return
		}

		if err := s.fsys.Chmod(f.Path, fs.FileMode(mode)); err != nil {
			cb.Invoke(jsutil.ToJSError(err))
			return
		}

		cb.Invoke(nil)
	}()

	return nil
}

// mkdir(path, perm, callback)
func (s *Service) mkdir(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	perm := args[1].Int()
	cb := args[2]

	go func() {
		log("mkdir", path)

		if err := s.fsys.MkdirAll(path, os.FileMode(perm)); err != nil {
			cb.Invoke(jsutil.ToJSError(err))
			return
		}
		cb.Invoke(nil)
	}()

	return nil
}

// rename(from, to, callback)
func (s *Service) rename(this js.Value, args []js.Value) any {
	from := cleanPath(args[0].String())
	to := cleanPath(args[1].String())
	cb := args[2]

	go func() {
		log("rename", from, to)

		if err := s.fsys.Rename(from, to); err != nil {
			cb.Invoke(jsutil.ToJSError(err))
			return
		}

		cb.Invoke(nil)
	}()

	return nil
}

// rmdir(path, callback)
func (s *Service) rmdir(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	cb := args[1]

	go func() {
		log("rmdir", path)

		// TODO: should only remove if dir is empty i think?
		if err := s.fsys.RemoveAll(path); err != nil {
			cb.Invoke(jsutil.ToJSError(err))
			return
		}

		cb.Invoke(nil)
	}()

	return nil
}

// unlink(path, callback)
func (s *Service) unlink(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	cb := args[1]

	go func() {
		log("unlink", path)

		if err := s.fsys.Remove(path); err != nil {
			cb.Invoke(jsutil.ToJSError(err))
			return
		}

		cb.Invoke(nil)
	}()

	return nil
}

// fsync(fd, callback)
func (s *Service) fsync(this js.Value, args []js.Value) any {
	fd := args[0].Int()
	cb := args[1]

	go func() {
		log("fsync", fd)

		s.mu.Lock()
		f, ok := s.fds[fd]
		s.mu.Unlock()
		if !ok {
			cb.Invoke(jsutil.ToJSError(syscall.EBADF))
			return
		}

		if sf, ok := f.File.(interface {
			Sync() error
		}); ok {
			if err := sf.Sync(); err != nil {
				cb.Invoke(jsutil.ToJSError(err))
				return
			}
		}

		cb.Invoke(nil)
	}()

	return nil
}

// utimes(path, atime, mtime, callback) { callback(enosys()); },
func (s *Service) utimes(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	atime := time.Unix(int64(args[1].Int()), 0)
	mtime := time.Unix(int64(args[2].Int()), 0)
	cb := args[3]

	go func() {
		log("utimes", path)

		if err := s.fsys.Chtimes(path, atime, mtime); err != nil {
			cb.Invoke(jsutil.ToJSError(err))
			return
		}

		cb.Invoke(nil)
	}()

	return nil
}
