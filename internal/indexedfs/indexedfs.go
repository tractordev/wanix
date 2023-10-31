package indexedfs

import (
	"errors"
	"io"
	"os"
	"syscall/js"
	"time"

	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/wanix/internal/jsutil"
)

type FS struct {
	db js.Value
}

func Initalize() (*FS, error) {
	// import dbhelper.js
	blob := js.Global().Get("initfs").Get("dbhelper.js")
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	_dbhelper = jsutil.Await(js.Global().Call("import", url))

	db := jsutil.Await(callHelper("initialize"))
	if db.Truthy() {
		return &FS{db: db}, nil
	} else {
		return nil, errors.New("Unable to open IndexedDB")
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

	// look for file in database
	// make sure we're using the right permissions
	// create a transaction using the appropriate flags
	// store the transaction in the file?
	// profit?

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
	panic("Stat unimplemented")
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

var _dbhelper js.Value = js.Undefined()

func callHelper(name string, args ...any) js.Value {
	// if _dbhelper.IsUndefined() {
	// 	_dbhelper = js.Global().Get("indexedfsHelper")
	// }
	jsutil.Log(name, args)
	return _dbhelper.Call(name, args)
}
