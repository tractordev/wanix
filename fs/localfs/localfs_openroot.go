//go:build !wasm && !tinygo

package localfs

import (
	"context"
	"log/slog"
	"os"
	"time"

	"tractor.dev/wanix/fs"
)

// NewWithVirtualUidGid creates a new localfs that virtualizes all uid/gid to 0:0
// and stores chown operations in memory instead of applying them to the filesystem
func NewWithVirtualUidGid(dir string) (*FS, error) {
	return newRoot(dir, &FS{
		virtualizeUidGid: true,
		chownData:        make(map[string][2]int),
		log:              slog.Default(), // for now
	})
}

func newRoot(dir string, fsys *FS) (*FS, error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	fsys.baseDir = r.Name()
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
