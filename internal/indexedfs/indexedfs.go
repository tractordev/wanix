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

// TODO: better fs errors

var helper js.Value = js.Undefined()

func callHelper(name string, args ...any) js.Value {
	//jsutil.Log(name, args)
	return helper.Call(name, args...)
}

type FS struct {
	db js.Value
}

func New() (*FS, error) {
	if helper.IsUndefined() {
		blob := js.Global().Get("initfs").Get("indexedfs.js")
		url := js.Global().Get("URL").Call("createObjectURL", blob)
		helper = jsutil.Await(js.Global().Call("import", url))
	}

	db, err := jsutil.AwaitErr(callHelper("initialize"))
	if err != nil {
		return nil, err
	}
	return &FS{db: db}, nil
}

func (ifs *FS) Chmod(name string, mode fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chmod", Path: name, Err: fs.ErrInvalid}
	}

	updateFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		file := args[0]
		file.Set("perms", uint32(mode.Perm()))
		file.Set("ctime", time.Now().Unix())
		return file
	})
	defer updateFunc.Release()

	if _, err := jsutil.AwaitErr(callHelper("updateFile", ifs.db, name, updateFunc)); err != nil {
		return &fs.PathError{Op: "chmod", Path: name, Err: err}
	}

	return nil
}
func (ifs *FS) Chown(name string, uid, gid int) error {
	return fs.ErrPermission // TODO: maybe just a no-op?
}
func (ifs *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chtimes", Path: name, Err: fs.ErrInvalid}
	}

	updateFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		file := args[0]
		file.Set("atime", atime.Unix())
		file.Set("mtime", mtime.Unix())
		file.Set("ctime", time.Now().Unix())
		return file
	})
	defer updateFunc.Release()

	if _, err := jsutil.AwaitErr(callHelper("updateFile", ifs.db, name, updateFunc)); err != nil {
		return &fs.PathError{Op: "chtimes", Path: name, Err: err}
	}

	return nil
}
func (ifs *FS) Create(name string) (fs.File, error) {
	return ifs.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
}
func (ifs *FS) Mkdir(name string, perm fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrInvalid}
	}
	dir := filepath.Dir(name)
	if dir != "." && dir != "/" {
		exists, err := fs.DirExists(ifs, dir)
		if err != nil {
			return &fs.PathError{Op: "mkdir", Path: name, Err: err}
		}
		if !exists {
			return &fs.PathError{Op: "mkdir", Path: dir, Err: fs.ErrInvalid}
		}
	}

	_, err := jsutil.AwaitErr(callHelper("addFile", ifs.db, name, uint32(perm), true, time.Now().Unix()))
	if err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: err}
	}

	return nil
}
func (ifs *FS) MkdirAll(path string, perm fs.FileMode) error {
	if !fs.ValidPath(path) {
		return &fs.PathError{Op: "mkdirAll", Path: path, Err: fs.ErrInvalid}
	}

	var pp []string
	for _, p := range strings.Split(path, "/") {
		if p == "" {
			continue
		}
		pp = append(pp, p)
		dir := filepath.Join(pp...)
		exists, err := fs.DirExists(ifs, dir)
		if err != nil {
			return &fs.PathError{Op: "mkdirall", Path: dir, Err: err}
		}
		if !exists {
			if err := ifs.Mkdir(dir, perm); err != nil {
				return &fs.PathError{Op: "mkdirall", Path: dir, Err: err}
			}
		}
	}
	return nil
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

	var (
		file fs.File = nil
		key  js.Value
		err  error = nil
	)

	key, err = jsutil.AwaitErr(callHelper("getFileKey", ifs.db, name))
	if err == nil {
		file = &indexedFile{name: name, key: key.Int(), ifs: ifs, flags: flag}
	}

	if err == nil && (flag&(os.O_EXCL|os.O_CREATE) == (os.O_EXCL | os.O_CREATE)) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrExist}
	}

	if err != nil {
		// TODO: figure out a better way of signaling error type from javascript
		if strings.Contains(err.Error(), "ErrNotExist") {
			if flag&os.O_CREATE > 0 {
				key, err = jsutil.AwaitErr(callHelper("addFile", ifs.db, name, uint32(perm), perm.IsDir(), time.Now().Unix()))
				if err != nil {
					return nil, &fs.PathError{Op: "open", Path: name, Err: err}
				}

				file = &indexedFile{name: name, key: key.Int(), ifs: ifs, flags: flag}
			} else {
				return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
			}
		} else {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
	}

	if flag&os.O_APPEND > 0 {
		file.(*indexedFile).Seek(0, io.SeekEnd)
	}
	if flag&os.O_TRUNC > 0 {
		// TODO: proper Truncate implementation
		file.(*indexedFile).Seek(0, io.SeekStart)
	}

	// if we didn't just create the file, update atime
	if flag&os.O_CREATE == 0 {
		updateFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
			file := args[0]
			file.Set("atime", time.Now().Unix())
			return file
		})
		defer updateFunc.Release()

		if _, err = jsutil.AwaitErr(callHelper("updateFile", ifs.db, name, updateFunc)); err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
	}

	return file, nil
}

func (ifs *FS) Remove(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
	}
	key, err := jsutil.AwaitErr(callHelper("getFileKey", ifs.db, name))
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}
	if _, err = jsutil.AwaitErr(callHelper("deleteFile", ifs.db, key)); err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}

	return nil
}

func (ifs *FS) RemoveAll(path string) error {
	if !fs.ValidPath(path) {
		return &fs.PathError{Op: "removeAll", Path: path, Err: fs.ErrInvalid}
	}
	if _, err := jsutil.AwaitErr(callHelper("deleteAll", ifs.db, path)); err != nil {
		return &fs.PathError{Op: "removeAll", Path: path, Err: err}
	}
	return nil
}

func (ifs *FS) Rename(oldname, newname string) error {
	if !fs.ValidPath(oldname) {
		return &fs.PathError{Op: "rename", Path: oldname, Err: fs.ErrInvalid}
	}
	if !fs.ValidPath(newname) {
		return &fs.PathError{Op: "rename", Path: newname, Err: fs.ErrInvalid}
	}

	key, err := jsutil.AwaitErr(callHelper("getFileKey", ifs.db, oldname))
	if err != nil {
		return &fs.PathError{Op: "rename", Path: newname, Err: err}
	}

	updateFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		file := args[0]
		file.Set("path", newname)
		file.Set("ctime", time.Now().Unix())
		return file
	})
	defer updateFunc.Release()

	if _, err = jsutil.AwaitErr(callHelper("updateFile", ifs.db, key, updateFunc)); err != nil {
		return &fs.PathError{Op: "rename", Path: newname, Err: err}
	}

	return nil
}

func (ifs *FS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	f, err := jsutil.AwaitErr(callHelper("getFileByPath", ifs.db, name))
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}

	return &indexedInfo{
		name:    filepath.Base(f.Get("path").String()),
		size:    int64(f.Get("size").Int()),
		isDir:   f.Get("isdir").Bool(),
		modTime: int64(f.Get("mtime").Int()),
	}, nil
}

func (ifs *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readDir", Path: name, Err: fs.ErrInvalid}
	}

	entries, err := jsutil.AwaitErr(callHelper("getDirEntries", ifs.db, name))
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
	}
	var fsEntries []fs.DirEntry
	for i := 0; i < entries.Length(); i++ {
		e := entries.Index(i)
		fsEntries = append(fsEntries, &indexedInfo{
			name:    filepath.Base(e.Get("path").String()),
			size:    int64(e.Get("size").Int()),
			isDir:   e.Get("isdir").Bool(),
			modTime: int64(e.Get("mtime").Int()),
		})
	}
	return fsEntries, nil
}

type indexedFile struct {
	name   string
	key    int
	ifs    *FS
	flags  int
	offset int64
	// TODO: see if read/write caches can be merged
	// used internally by getData()
	readCache    []byte
	outdatedRead bool
	// used internally by Write() and Sync()
	writeCache []byte
	dirty      bool
}

func (f *indexedFile) ReadDir(n int) ([]fs.DirEntry, error) {
	return f.ifs.ReadDir(f.name)
}

func (f *indexedFile) getData() ([]byte, error) {
	if f.outdatedRead || f.readCache == nil {
		// Deliberately not updating atime, as that can get slow fast.
		// Aiming for posix-like, not compliant.
		data, err := jsutil.AwaitErr(callHelper("readFile", f.ifs.db, f.key))
		if err != nil {
			return nil, err
		}

		f.outdatedRead = false
		if data.IsNull() {
			f.readCache = []byte{}
		} else {
			f.readCache = make([]byte, data.Length())
			js.CopyBytesToGo(f.readCache, data)
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

	data, err := f.getData()
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
		if f.writeCache, err = f.getData(); err != nil {
			return 0, err
		}
	}

	writeEnd := f.offset + int64(len(p))

	if writeEnd > int64(cap(f.writeCache)) {
		newCapacity := int64(cap(f.writeCache))*2 + 1
		for ; writeEnd > newCapacity; newCapacity *= 2 {
		}

		newCache := make([]byte, len(f.writeCache), newCapacity)
		copy(newCache, f.writeCache)
		f.writeCache = newCache
	}

	copy(f.writeCache[f.offset:writeEnd], p)
	f.writeCache = f.writeCache[:writeEnd]
	f.offset = writeEnd
	f.dirty = true
	return len(p), nil
}

func (f *indexedFile) Sync() error {
	if !f.dirty {
		return nil
	}

	updateFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		file := args[0]

		mime := map[string]interface{}{
			"type": "application/octet-stream",
		}

		buf := js.Global().Get("Uint8Array").New(len(f.writeCache))
		js.CopyBytesToJS(buf, f.writeCache)
		file.Set("blob", js.Global().Get("Blob").New([]any{buf}, mime))
		file.Set("size", len(f.writeCache))

		file.Set("mtime", time.Now().Unix())
		file.Set("ctime", time.Now().Unix())
		file.Set("atime", time.Now().Unix())
		return file
	})
	defer updateFunc.Release()

	_, err := jsutil.AwaitErr(callHelper("updateFile", f.ifs.db, f.key, updateFunc))
	if err != nil {
		return &fs.PathError{Op: "sync", Path: f.name, Err: err}
	}

	f.dirty = false
	f.outdatedRead = true
	return nil
}

// Stat implements fs.File.
func (f *indexedFile) Stat() (fs.FileInfo, error) {
	return f.ifs.Stat(f.name)
}

// Stat implements fs.File.
func (f *indexedFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.offset = offset
	case io.SeekCurrent:
		f.offset += offset
	case io.SeekEnd:
		if d, err := f.getData(); err == nil {
			f.offset = int64(len(d)) + offset
		} else {
			return 0, &fs.PathError{Op: "seek", Path: f.name, Err: err}
		}
	}
	if f.offset < 0 {
		f.offset = 0
		return 0, &fs.PathError{Op: "seek", Path: f.name, Err: fmt.Errorf("%w: resultant offset cannot be negative", fs.ErrInvalid)}
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
		return 0755 | os.ModeDir
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

// these allow it to act as DirInfo as well
func (i *indexedInfo) Info() (fs.FileInfo, error) {
	return i, nil
}
func (i *indexedInfo) Type() fs.FileMode {
	return i.Mode()
}
