//go:build !wasm

package localfs

import (
	"context"
	"os"
	"time"

	"tractor.dev/wanix/fs"
)

func newRoot(dir string, fsys *FS) (*FS, error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	fsys.root = r
	fsys.create = func(name string) (fs.File, error) {
		return r.Create(name)
	}
	fsys.mkdir = func(name string, perm fs.FileMode) error {
		return r.Mkdir(name, perm)
	}
	fsys.mkdirAll = func(path string, perm fs.FileMode) error {
		return r.MkdirAll(path, perm)
	}
	fsys.openContext = func(ctx context.Context, name string) (fs.File, error) {
		return r.Open(name)
	}
	fsys.openFile = func(name string, flag int, perm fs.FileMode) (fs.File, error) {
		return r.OpenFile(name, flag, perm)
	}
	fsys.remove = func(name string) error {
		return r.Remove(name)
	}
	fsys.removeAll = func(path string) error {
		return r.RemoveAll(path)
	}
	fsys.rename = func(oldname, newname string) error {
		return r.Rename(oldname, newname)
	}
	fsys.stat = func(name string) (fs.FileInfo, error) {
		return r.Stat(name)
	}
	fsys.lstat = func(name string) (fs.FileInfo, error) {
		return r.Lstat(name)
	}
	fsys.chmod = func(name string, mode fs.FileMode) error {
		return r.Chmod(name, mode)
	}
	fsys.chown = func(name string, uid, gid int) error {
		return r.Chown(name, uid, gid)
	}
	fsys.chtimes = func(name string, atime time.Time, mtime time.Time) error {
		return r.Chtimes(name, atime, mtime)
	}
	fsys.symlink = func(oldname, newname string) error {
		return r.Symlink(oldname, newname)
	}
	fsys.readlink = func(name string) (string, error) {
		return r.Readlink(name)
	}
	return fsys, nil
}
