//go:build js && wasm

package fsa

import (
	"context"
	"log"
	"path"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"github.com/fxamacker/cbor/v2"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/web/jsutil"
)

type FS js.Value

type Stat struct {
	Name  string
	Size  uint64
	Mode  fs.FileMode
	Atime time.Time
	Mtime time.Time
}

func (s Stat) Info() fs.FileInfo {
	return fskit.Entry(path.Base(s.Name), s.Mode, s.Size, s.Mtime)
}

var statCache sync.Map

// todo: clean all this up!
func statStore(fsys FS, name string, stat Stat) {
	statCache.Store(name, stat)
	go func() {
		var stats []Stat
		statCache.Range(func(key, value any) bool {
			stats = append(stats, value.(Stat))
			return true
		})
		b, err := cbor.Marshal(stats)
		if err != nil {
			log.Println("fsa: statstore: marshal:", err)
			return
		}
		if err := fs.WriteFile(fsys, "#stat", b, 0755); err != nil {
			log.Println("fsa: statstore: write:", err)
		}
	}()
}

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

func (fsys FS) Symlink(oldname, newname string) error {
	if !fs.ValidPath(newname) {
		return &fs.PathError{Op: "symlink", Path: newname, Err: fs.ErrInvalid}
	}

	err := fs.WriteFile(fsys, newname, []byte(oldname), 0777)
	if err != nil {
		return err
	}

	v, ok := statCache.Load(newname)
	if ok {
		stat := v.(Stat)
		stat.Mode = fs.FileMode(0777) | fs.ModeSymlink
		statStore(fsys, newname, stat)
	} else {
		statStore(fsys, newname, Stat{Name: newname, Mode: fs.FileMode(0777) | fs.ModeSymlink})
	}

	return nil
}

func (fsys FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chtimes", Path: name, Err: fs.ErrInvalid}
	}

	v, ok := statCache.Load(name)
	if ok {
		stat := v.(Stat)
		// stat.atime = atime
		stat.Mtime = mtime
		statStore(fsys, name, stat)
		return nil
	}
	statStore(fsys, name, Stat{Name: name, Atime: atime, Mtime: mtime})

	return nil
}

func (fsys FS) Chmod(name string, mode fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chmod", Path: name, Err: fs.ErrInvalid}
	}

	v, ok := statCache.Load(name)
	if ok {
		stat := v.(Stat)
		// Keep the file type bits and update only the permission bits
		stat.Mode = (stat.Mode & fs.ModeType) | (mode & fs.ModePerm)
		statStore(fsys, name, stat)
		return nil
	}
	statStore(fsys, name, Stat{Name: name, Mode: mode & fs.ModePerm})
	return nil
}

func (fsys FS) Stat(name string) (fs.FileInfo, error) {
	return fsys.StatContext(context.Background(), name)
}

func (fsys FS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	f, err := fsys.OpenContext(fs.WithNoFollow(ctx), name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
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
		v, ok := statCache.Load(name)
		if ok && fs.FollowSymlinks(ctx) && fs.IsSymlink(v.(Stat).Mode) {
			if origin, fullname, ok := fs.Origin(ctx); ok {
				target, err := fs.Readlink(fsys, name)
				if err != nil {
					return nil, err
				}
				if strings.HasPrefix(target, "/") {
					target = target[1:]
				} else {
					target = path.Join(strings.TrimSuffix(fullname, name), target)
				}
				return fs.OpenContext(ctx, origin, target)
			} else {
				log.Println("fsa: opencontext: no origin for symlink:", name)
				return nil, fs.ErrInvalid
			}
		}
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

func (fsys FS) Rename(oldname, newname string) error {
	if !fs.ValidPath(oldname) || !fs.ValidPath(newname) {
		return &fs.PathError{Op: "rename", Path: oldname, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, oldname)
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "rename", Path: oldname, Err: fs.ErrNotExist}
	}

	ok, err = fs.Exists(fsys, newname)
	if err != nil {
		return err
	}
	if ok {
		return &fs.PathError{Op: "rename", Path: newname, Err: fs.ErrExist}
	}

	oldDirHandle, err := fsys.walkDir(path.Dir(oldname))
	if err != nil {
		return &fs.PathError{Op: "rename", Path: oldname, Err: fs.ErrNotExist}
	}

	newDirHandle, err := fsys.walkDir(path.Dir(newname))
	if err != nil {
		return &fs.PathError{Op: "rename", Path: newname, Err: fs.ErrNotExist}
	}

	if err := fs.CopyAll(fsys, oldname, newname); err != nil {
		return err
	}

	_, err = jsutil.AwaitErr(oldDirHandle.Call("removeEntry", path.Base(oldname), map[string]any{"recursive": true}))
	if err != nil {
		// Try to clean up the copy if delete fails
		newDirHandle.Call("removeEntry", path.Base(newname), map[string]any{"recursive": true})
		return err
	}

	statCache.Delete(oldname)
	return nil
}
