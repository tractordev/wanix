//go:build js && wasm

package fsa

import (
	"context"
	"path"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/web/jsutil"
)

type FS js.Value

type stat struct {
	name  string
	size  uint64
	mode  fs.FileMode
	atime time.Time
	mtime time.Time
}

func (s stat) Info() fs.FileInfo {
	return fskit.Entry(path.Base(s.name), s.mode, s.size, s.mtime)
}

var statCache sync.Map

func (fsys FS) walkDir(path string) (js.Value, error) {
	if path == "." {
		return js.Value(fsys), nil
	}
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	cur := js.Value(fsys)
	var err error
	for i := 0; i < len(parts); i++ {
		cur, err = jsutil.AwaitErr(cur.Call("getDirectoryHandle", parts[i], map[string]any{"create": false}))
		if err != nil {
			return js.Undefined(), err
		}
	}
	return cur, nil
}

// Chtimes is not supported but implement as no-op
func (fsys FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chtimes", Path: name, Err: fs.ErrInvalid}
	}

	v, ok := statCache.Load(name)
	if ok {
		stat := v.(stat)
		// stat.atime = atime
		stat.mtime = mtime
		statCache.Store(name, stat)
		return nil
	}
	statCache.Store(name, stat{atime: atime, mtime: mtime})

	return nil
}

func (fsys FS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

func (fsys FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	if name == "." {
		return DirHandleFile(fsys, name, js.Value(fsys)), nil
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return nil, fs.ErrNotExist
	}

	file, err := jsutil.AwaitErr(dirHandle.Call("getFileHandle", path.Base(name), map[string]any{"create": false}))
	if err == nil {
		return NewFileHandle(name, file, true), nil
	}

	dir, err := jsutil.AwaitErr(dirHandle.Call("getDirectoryHandle", path.Base(name), map[string]any{"create": false}))
	if err == nil {
		return DirHandleFile(fsys, name, dir), nil
	}

	return nil, fs.ErrNotExist
}

func (fsys FS) Truncate(name string, size int64) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "truncate", Path: name, Err: fs.ErrNotExist}
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return fs.ErrNotExist
	}

	file, err := jsutil.AwaitErr(dirHandle.Call("getFileHandle", path.Base(name), map[string]any{"create": false}))
	if err == nil {
		return NewFileHandle(name, file, false).Truncate(size)
	}

	return fs.ErrNotExist
}

func (fsys FS) Create(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrNotExist}
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return nil, fs.ErrNotExist
	}

	handle, err := jsutil.AwaitErr(dirHandle.Call("getFileHandle", path.Base(name), map[string]any{"create": true}))
	if err != nil {
		return nil, err
	}
	return NewFileHandle(name, handle, false), nil
}

func (fsys FS) Mkdir(name string, perm fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if ok {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrExist}
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
	}

	_, err = jsutil.AwaitErr(dirHandle.Call("getDirectoryHandle", path.Base(name), map[string]any{"create": true}))
	return err
}

func (fsys FS) Remove(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	if isDir, err := fs.IsDir(fsys, name); err != nil {
		return err
	} else if isDir {
		empty, err := fs.IsEmpty(fsys, name)
		if err != nil {
			return err
		}
		if !empty {
			return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotEmpty}
		}
	}

	dirHandle, err := fsys.walkDir(path.Dir(name))
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	_, err = jsutil.AwaitErr(dirHandle.Call("removeEntry", path.Base(name)))
	if err != nil {
		return err
	}
	statCache.Delete(name)
	return nil
}
