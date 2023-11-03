package indexedfs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall/js"
	"time"

	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/wanix/internal/jsutil"
)

type FS struct {
	db js.Value
}

func Initalize() (*FS, error) {
	if dbhelper.IsUndefined() {
		// import dbhelper.js
		blob := js.Global().Get("initfs").Get("dbhelper.js")
		url := js.Global().Get("URL").Call("createObjectURL", blob)
		dbhelper = jsutil.Await(js.Global().Call("import", url))
	}

	db, err := jsutil.AwaitAll(callHelper("initialize"))
	if err.Truthy() {
		return nil, js.Error{err}
	} else {
		return &FS{db: db}, nil
	}
}

func (ifs *FS) Chmod(name string, mode fs.FileMode) error {
	panic("Chmod unimplemented")
}
func (ifs *FS) Chown(name string, uid, gid int) error {
	panic("Chown unimplemented")
}
func (ifs *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	updateFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		file := args[0]
		file.Set("atime", atime.Unix())
		file.Set("mtime", mtime.Unix())
		return file
	})
	defer updateFunc.Release()

	err := jsutil.Await(callHelper("updateFile", ifs.db, name, updateFunc))
	if err.Truthy() {
		return js.Error{err}
	} else {
		return nil
	}
}
func (ifs *FS) Create(name string) (fs.File, error) {
	return ifs.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
}
func (ifs *FS) Mkdir(name string, perm fs.FileMode) error {
	panic("Mkdir unimplemented")
}
func (ifs *FS) MkdirAll(path string, perm fs.FileMode) error {
	panic("MkdirAll unimplemented")
}

func (ifs *FS) Open(name string) (fs.File, error) {
	return ifs.OpenFile(name, os.O_RDONLY, 0)
}

func (ifs *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	if flag&os.O_SYNC > 0 {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("%w O_SYNC", errors.ErrUnsupported)}
	}

	// TODO:
	// make sure we're using the right permissions
	// profit?

	var file fs.File = nil
	var err error = nil

	if key, jsErr := jsutil.AwaitAll(callHelper("getFileKey", ifs.db, name)); jsErr.Truthy() {
		err = js.Error{jsErr}
	} else {
		file = &indexedFile{key: key.Int(), ifs: ifs, flags: flag}
	}

	if err == nil && (flag&(os.O_EXCL|os.O_CREATE) == (os.O_EXCL | os.O_CREATE)) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrExist}
	}

	// TODO: figure out a better way of signaling error type from javascript
	if err != nil && strings.Contains(err.Error(), "ErrNotExist") && (flag&os.O_CREATE > 0) {
		if key, jsErr := jsutil.AwaitAll(callHelper("addFile", ifs.db, name, uint32(perm), perm.IsDir())); jsErr.Truthy() {
			err = js.Error{jsErr}
		} else {
			file = &indexedFile{key: key.Int(), ifs: ifs, flags: flag}
			err = nil
		}
	}

	// TODO: fully convert js errors to Go errors
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	if flag&os.O_APPEND > 0 {
		file.(*indexedFile).Seek(0, io.SeekEnd)
	}
	if flag&os.O_TRUNC > 0 {
		// TODO: proper Truncate implementation
		file.(*indexedFile).Seek(0, io.SeekStart)
	}

	return file, nil
}

func (ifs *FS) Remove(name string) error {
	panic("Remove unimplemented")
}
func (ifs *FS) RemoveAll(path string) error {
	panic("RemoveAll unimplemented")
}
func (ifs *FS) Rename(oldname, newname string) error {
	panic("Rename unimplemented")
}

func (ifs *FS) Stat(name string) (fs.FileInfo, error) {
	f, err := jsutil.AwaitAll(callHelper("getFileByPath", ifs.db, name))
	if err.Truthy() {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: js.Error{err}}
	}

	return &indexedInfo{
		name:    filepath.Base(f.Get("path").String()),
		size:    int64(f.Get("size").Int()),
		isDir:   f.Get("isdir").Bool(),
		modTime: int64(f.Get("mtime").Int()),
	}, nil
}

type indexedFile struct {
	key    int
	ifs    *FS
	flags  int
	offset int64
	// TODO: see if read/write caches can be merged
	// used internally by data()
	readCache    []byte
	outdatedRead bool
	// used internally by Write() and Sync()
	writeCache []byte
	dirty      bool
}

func (f *indexedFile) data() ([]byte, error) {
	if f.outdatedRead || f.readCache == nil {
		array, err := jsutil.AwaitAll(callHelper("readFile", f.ifs.db, f.key))
		if err.Truthy() {
			return nil, js.Error{err}
		}

		f.outdatedRead = false
		if array.IsNull() {
			f.readCache = []byte{}
		} else {
			f.readCache = jsutil.ToGoByteSlice(array)
		}
	}

	return f.readCache, nil
}

// Close implements fs.File.
func (f *indexedFile) Close() error {
	f.offset = 0

	if f.dirty {
		return f.Sync()
	}
	// f.readCache = nil
	// f.ifs = nil
	// f = nil
	return nil
}

// Read implements fs.File.
func (f *indexedFile) Read(p []byte) (n int, err error) {
	if f.flags&os.O_WRONLY > 0 {
		return 0, fs.ErrPermission
	}

	data, err := f.data()
	if err != nil {
		return 0, err
	}

	if f.offset >= int64(len(data)) {
		return 0, io.EOF
	}

	rest := data[f.offset:]
	if len(rest) < len(p) {
		n = len(rest)
	} else {
		n = len(p)
	}

	copy(p, rest[:n])
	f.offset += int64(n)
	return n, nil
}

func (f *indexedFile) Write(p []byte) (n int, err error) {
	if f.writeCache == nil {
		if f.writeCache, err = f.data(); err != nil {
			return 0, err
		}
	}

	writeEnd := f.offset + int64(len(p))

	if writeEnd > int64(cap(f.writeCache)) {
		newCapacity := cap(f.writeCache)*2 + 1
		for ; writeEnd > int64(newCapacity); newCapacity *= 2 {
		}

		newCache := make([]byte, len(f.writeCache), newCapacity)
		copy(newCache, f.writeCache)
		f.writeCache = newCache
	}

	copy(f.writeCache[f.offset:writeEnd], p)
	f.writeCache = f.writeCache[:writeEnd]
	f.dirty = true
	jsutil.Log("Write:", jsutil.ToJSArray(f.writeCache))
	return len(p), nil
}

func (f *indexedFile) Sync() error {
	jsutil.Log("Sync")
	if !f.dirty {
		return nil
	}

	updateFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		file := args[0]

		mime := map[string]interface{}{
			"type": "application/octet-stream",
		}

		file.Set("blob", js.Global().Get("Blob").New(jsutil.ToJSArray(f.writeCache), mime))
		file.Set("size", len(f.writeCache))
		// TODO: set mtime
		return file
	})
	defer updateFunc.Release()

	err := jsutil.Await(callHelper("updateFile", f.ifs.db, f.key, updateFunc))
	if err.Truthy() {
		return js.Error{err}
	}

	f.dirty = false
	f.outdatedRead = true
	return nil
}

// Stat implements fs.File.
func (f *indexedFile) Stat() (fs.FileInfo, error) {
	panic("Stat unimplemented")
}

// Stat implements fs.File.
func (f *indexedFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.offset = offset
	case io.SeekCurrent:
		f.offset += offset
	case io.SeekEnd:
		if d, err := f.data(); err == nil {
			f.offset = int64(len(d)) + offset
		} else {
			return 0, err
		}
	}
	if f.offset < 0 {
		f.offset = 0
		return 0, fmt.Errorf("Seek: resultant offset cannot be negative")
	}
	return f.offset, nil
}

type indexedInfo struct {
	name    string
	size    int64
	isDir   bool
	modTime int64
}

func (i *indexedInfo) Name() string {
	return i.name
}

func (i *indexedInfo) Size() int64 {
	return i.size
}

func (i *indexedInfo) Mode() fs.FileMode {
	if i.isDir {
		return 0755 | fs.ModeDir
	}
	return 0644
}

func (i *indexedInfo) ModTime() time.Time {
	return time.Unix(i.modTime, 0)
}

func (i *indexedInfo) IsDir() bool {
	return i.isDir
}

func (i *indexedInfo) Sys() any {
	return nil
}

var dbhelper js.Value = js.Undefined()

func callHelper(name string, args ...any) js.Value {
	jsutil.Log(name, args)
	return dbhelper.Call(name, args...)
}
