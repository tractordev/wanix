//go:build !wasm

package localfs

import (
	"context"
	"os"
	"time"

	"tractor.dev/wanix/fs"
)

type FS struct {
	root *os.Root
}

func New(dir string) (*FS, error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &FS{
		root: r,
	}, nil
}

func (fsys *FS) Create(name string) (fs.File, error) {
	f, e := fsys.root.Create(name)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (fsys *FS) Mkdir(name string, perm fs.FileMode) error {
	return fsys.root.Mkdir(name, perm)
}

func (fsys *FS) MkdirAll(path string, perm fs.FileMode) error {
	return fsys.root.MkdirAll(path, perm)
}

func (fsys *FS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

func (fsys *FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	f, e := fsys.root.Open(name)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (fsys *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	f, e := fsys.root.OpenFile(name, flag, perm)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (fsys *FS) Remove(name string) error {
	return fsys.root.Remove(name)
}

func (fsys *FS) RemoveAll(path string) error {
	return fsys.root.RemoveAll(path)
}

func (fsys *FS) Rename(oldname, newname string) error {
	return fsys.root.Rename(oldname, newname)
}

func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	return fsys.StatContext(context.Background(), name)
}

func (fsys *FS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	if fs.FollowSymlinks(ctx) {
		return fsys.root.Stat(name)
	}
	return fsys.root.Lstat(name)
}

func (fsys *FS) Chmod(name string, mode fs.FileMode) error {
	return fsys.root.Chmod(name, mode)
}

func (fsys *FS) Chown(name string, uid, gid int) error {
	return fsys.root.Chown(name, uid, gid)
}

func (fsys *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return fsys.root.Chtimes(name, atime, mtime)
}

func (fsys *FS) Symlink(oldname string, newname string) error {
	return fsys.root.Symlink(oldname, newname)
}

func (fsys *FS) Readlink(name string) (string, error) {
	return fsys.root.Readlink(name)
}
