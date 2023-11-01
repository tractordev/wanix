package fs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"syscall/js"
	"time"

	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/wanix/internal/indexedfs"
	"tractor.dev/wanix/internal/jsutil"

	"tractor.dev/toolkit-go/engine/fs/fsutil"
	"tractor.dev/toolkit-go/engine/fs/mountablefs"
)

func log(args ...any) {
	js.Global().Get("console").Call("log", args...)
}

type Service struct {
	fsys fs.MutableFS

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
	if ifs, err := indexedfs.Initalize(); err != nil {
		panic(err)
	} else {
		s.fsys = mountablefs.New(ifs)
	}

	// fsutil.MkdirAll(s.fsys, "debug", 0755)

	if f, err := s.fsys.Open("home/hello.txt"); err == nil {
		data := make([]byte, 32)
		if n, err := f.Read(data); err == nil {
			fmt.Printf("%s\n", data[:n])
		} else {
			jsutil.Err(err)
		}
	} else {
		jsutil.Err(err)
	}
	if f, err := s.fsys.Open("home/goodbye.txt"); err == nil {
		data := make([]byte, 32)
		if n, err := f.Read(data); err == nil {
			fmt.Printf("%s\n", data[:n])
		} else {
			jsutil.Err(err)
		}
	} else {
		jsutil.Err(err)
	}
	fsutil.WriteFile(s.fsys, "debug.txt", []byte("Hello world"), 0644)
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

	fsObj.Set("stdinWrite", js.FuncOf(func(this js.Value, args []js.Value) any {
		size := args[0].Length()
		log("stdinWrite", size)
		// not sure why we can't just use the Uint8Array passed in,
		// CopyBytesToGo will complain its not a Uint8Array, so we
		// make a fresh one and copy into it and it seems to work
		jsbuf := js.Global().Get("Uint8Array").New(size)
		jsbuf.Call("set", args[0], 0)
		buf := make([]byte, size)
		js.CopyBytesToGo(buf, jsbuf)
		if _, err := StdinBuf.Write(buf); err != nil {
			log("stdinWrite:", err.Error())
		}
		return nil
	}))
}

var StdinBuf = NewDataBuffer()

func cleanPath(path string) string {
	p := strings.TrimLeft(filepath.Clean(path), "/")
	if p == "" {
		return "/"
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
			cb.Invoke(jsError(err))
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
		// 		cb.Invoke(jsError(err), 0)
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
			cb.Invoke(jsError(syscall.EBADF), 0)
			return
		}

		if rs, ok := f.File.(io.ReadSeeker); ok && !pos.IsNull() {
			_, err := rs.Seek(int64(pos.Int()), 0)
			if err != nil {
				cb.Invoke(jsError(err), 0)
				return
			}
		}

		buf := make([]byte, length)
		n, err := f.Read(buf)
		if n > 0 {
			js.CopyBytesToJS(jsbuf, buf[:n])
		}
		if err != nil && err != io.EOF {
			cb.Invoke(jsError(err), n)
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
			cb.Invoke(jsError(syscall.EBADF))
			return
		}

		if ws, ok := f.File.(io.WriteSeeker); ok && !pos.IsNull() {
			_, err := ws.Seek(int64(pos.Int()), 0)
			if err != nil {
				cb.Invoke(jsError(err))
				return
			}
		}

		buf := make([]byte, length)
		js.CopyBytesToGo(buf, jsbuf)

		if fw, ok := f.File.(io.Writer); ok {
			n, err := fw.Write(buf)
			if err != nil {
				cb.Invoke(jsError(err))
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
			cb.Invoke(jsError(err))
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
		cb.Invoke(jsError(err))
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
			cb.Invoke(jsError(syscall.EBADF))
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
			cb.Invoke(jsError(syscall.EBADF))
			return
		}

		if err := f.Close(); err != nil {
			cb.Invoke(jsError(err))
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
	log("chown", path)

	if err := s.fsys.Chown(path, uid, gid); err != nil {
		cb.Invoke(jsError(err))
		return nil
	}

	cb.Invoke(nil)
	return nil
}

// fchown(fd, uid, gid, callback)
func (s *Service) fchown(this js.Value, args []js.Value) any {
	fd := args[0].Int()
	uid := args[1].Int()
	gid := args[2].Int()
	cb := args[3]
	log("fchown", fd)

	s.mu.Lock()
	f, ok := s.fds[fd]
	s.mu.Unlock()
	if !ok {
		cb.Invoke(jsError(syscall.EBADF))
		return nil
	}

	if err := s.fsys.Chown(f.Path, uid, gid); err != nil {
		cb.Invoke(jsError(err))
		return nil
	}

	cb.Invoke(nil)
	return nil
}

// lchown(path, uid, gid, callback)
func (s *Service) lchown(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	uid := args[1].Int()
	gid := args[2].Int()
	cb := args[3]
	log("lchown", path)

	if err := s.fsys.Chown(path, uid, gid); err != nil {
		cb.Invoke(jsError(err))
		return nil
	}

	cb.Invoke(nil)
	return nil
}

// chmod(path, mode, callback)
func (s *Service) chmod(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	mode := args[1].Int()
	cb := args[2]
	log("chmod", path)

	if err := s.fsys.Chmod(path, fs.FileMode(mode)); err != nil {
		cb.Invoke(jsError(err))
		return nil
	}

	cb.Invoke(nil)
	return nil
}

// fchmod(fd, mode, callback)
func (s *Service) fchmod(this js.Value, args []js.Value) any {
	fd := args[0].Int()
	mode := args[1].Int()
	cb := args[2]
	log("fchmod", fd)

	s.mu.Lock()
	f, ok := s.fds[fd]
	s.mu.Unlock()
	if !ok {
		cb.Invoke(jsError(syscall.EBADF))
		return nil
	}

	if err := s.fsys.Chmod(f.Path, fs.FileMode(mode)); err != nil {
		cb.Invoke(jsError(err))
		return nil
	}

	cb.Invoke(nil)
	return nil
}

// mkdir(path, perm, callback)
func (s *Service) mkdir(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	perm := args[1].Int()
	cb := args[2]
	log("mkdir", path)

	go func() {
		if err := s.fsys.MkdirAll(path, os.FileMode(perm)); err != nil {
			cb.Invoke(jsError(err))
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
	log("rename", from, to)

	if err := s.fsys.Rename(from, to); err != nil {
		cb.Invoke(jsError(err))
		return nil
	}

	cb.Invoke(nil)
	return nil
}

// rmdir(path, callback)
func (s *Service) rmdir(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	cb := args[1]
	log("rmdir", path)

	// TODO: should only remove if dir is empty i think?
	if err := s.fsys.RemoveAll(path); err != nil {
		cb.Invoke(jsError(err))
		return nil
	}

	cb.Invoke(nil)
	return nil
}

// unlink(path, callback)
func (s *Service) unlink(this js.Value, args []js.Value) any {
	path := cleanPath(args[0].String())
	cb := args[1]
	log("unlink", path)

	if err := s.fsys.Remove(path); err != nil {
		cb.Invoke(jsError(err))
		return nil
	}

	cb.Invoke(nil)
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
			cb.Invoke(jsError(syscall.EBADF))
			return
		}

		if sf, ok := f.File.(interface {
			Sync() error
		}); ok {
			if err := sf.Sync(); err != nil {
				cb.Invoke(jsError(err))
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
			cb.Invoke(jsError(err))
			return
		}

		cb.Invoke(nil)
	}()

	return nil
}

func jsError(err error) js.Value {
	jsErr := js.Global().Get("Error").New(err.Error())
	if sysErr, ok := err.(syscall.Errno); ok {
		jsErr.Set("code", errnoString(sysErr))
	}
	// I guess fs errors arent syscall errors
	if errors.Is(err, fs.ErrClosed) {
		jsErr.Set("code", "EIO") // not sure on this one
	}
	if errors.Is(err, fs.ErrExist) {
		jsErr.Set("code", "EEXIST")
	}
	if errors.Is(err, fs.ErrInvalid) {
		jsErr.Set("code", "EINVAL")
	}
	if errors.Is(err, fs.ErrNotExist) {
		jsErr.Set("code", "ENOENT")
	}
	if errors.Is(err, fs.ErrPermission) {
		jsErr.Set("code", "EPERM")
	}
	//log("jserr:", err.Error(), jsErr.Get("code").String())
	return jsErr
}

/////

type DataBuffer struct {
	buf    *bytes.Buffer
	cond   *sync.Cond
	closed bool
}

func NewDataBuffer() *DataBuffer {
	return &DataBuffer{
		buf:  &bytes.Buffer{},
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

func (db *DataBuffer) Write(data []byte) (n int, err error) {
	db.cond.L.Lock()
	defer db.cond.L.Unlock()

	if db.closed {
		return 0, fmt.Errorf("buffer closed")
	}

	n, err = db.buf.Write(data)
	db.cond.Broadcast() // Signal that data has been written
	return
}

func (db *DataBuffer) Read(p []byte) (n int, err error) {
	db.cond.L.Lock()
	defer db.cond.L.Unlock()

	for db.buf.Len() == 0 && !db.closed {
		db.cond.Wait() // Wait for data to be written
	}

	if db.closed {
		return 0, fmt.Errorf("buffer closed")
	}

	return db.buf.Read(p)
}

func (db *DataBuffer) Close() {
	db.cond.L.Lock()
	defer db.cond.L.Unlock()

	db.closed = true
	db.cond.Broadcast() // Signal that the buffer is closed
}
