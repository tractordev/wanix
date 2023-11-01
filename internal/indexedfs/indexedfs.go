package indexedfs

import (
	"io"
	"os"
	"path/filepath"
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
	panic("Chtimes unimplemented")
}
func (ifs *FS) Create(name string) (fs.File, error) {
	panic("Create unimplemented")
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

	// TODO:
	// make sure we're using the right permissions
	// implement flags
	// profit?

	// store flags and db-key in File
	key, err := jsutil.AwaitAll(callHelper("getFileKey", ifs.db, name))
	if err.Truthy() {
		return nil, &fs.PathError{Op: "open", Path: name, Err: js.Error{err}}
	}

	return &indexedFile{key: key.Int(), ifs: ifs, flags: flag}, nil
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
	key       int
	ifs       *FS
	flags     int
	offset    int64
	readCache []byte
}

func (f *indexedFile) data() ([]byte, error) {
	if f.readCache == nil {
		array, err := jsutil.AwaitAll(callHelper("readFile", f.ifs.db, f.key))
		if err.Truthy() {
			return nil, js.Error{err}
		}
		f.readCache = jsutil.ToGoByteSlice(array)
	}

	return f.readCache, nil
}

// Close implements fs.File.
func (f *indexedFile) Close() error {
	f.readCache = nil
	f.ifs = nil
	f = nil
	return nil
}

// Read implements fs.File.
func (f *indexedFile) Read(b []byte) (int, error) {
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
	n := 0
	if len(rest) < len(b) {
		n = len(rest)
	} else {
		n = len(b)
	}

	copy(b, rest[:n])
	f.offset += int64(n)
	return n, nil
}

// Stat implements fs.File.
func (f *indexedFile) Stat() (fs.FileInfo, error) {
	panic("Stat unimplemented")
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
