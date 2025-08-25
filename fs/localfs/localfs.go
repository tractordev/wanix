//go:build !wasm

package localfs

import (
	"context"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/sys/unix"
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

func (fsys *FS) SetXattr(ctx context.Context, name string, attr string, data []byte, flags int) error {
	var op func(path string, attr string, data []byte, flags int) (err error)
	if fs.FollowSymlinks(ctx) {
		op = unix.Setxattr
	} else {
		op = unix.Lsetxattr
	}

	p := path.Join(fsys.root.Name(), name)
	return op(p, attr, data, flags)
}

func (fsys *FS) GetXattr(ctx context.Context, name string, attr string) ([]byte, error) {
	var op func(path string, attr string, dest []byte) (sz int, err error)
	if fs.FollowSymlinks(ctx) {
		op = unix.Getxattr
	} else {
		op = unix.Lgetxattr
	}

	p := path.Join(fsys.root.Name(), name)
	sz, err := op(p, attr, nil)
	if err != nil {
		return nil, &fs.PathError{Op: "getxattr-get-size", Path: p, Err: err}
	}

	b := make([]byte, sz)
	sz, err = op(p, attr, b)
	if err != nil {
		return nil, &fs.PathError{Op: "getxattr", Path: p, Err: err}
	}
	return b[:sz], nil
}

func (fsys *FS) ListXattrs(ctx context.Context, name string) ([]string, error) {
	var op func(path string, dest []byte) (sz int, err error)
	if fs.FollowSymlinks(ctx) {
		op = unix.Listxattr

	} else {
		op = unix.Llistxattr
	}

	p := path.Join(fsys.root.Name(), name)
	sz, err := op(p, nil)
	if err != nil {
		return nil, &fs.PathError{Op: "listxattr-get-size", Path: p, Err: err}
	}

	b := make([]byte, sz)
	sz, err = op(p, b)
	if err != nil {
		return nil, &fs.PathError{Op: "listxattr", Path: p, Err: err}
	}

	return strings.Split(strings.Trim(string(b[:sz]), "\000"), "\000"), nil
}

func (fsys *FS) RemoveXattr(ctx context.Context, name string, attr string) error {
	var op func(path string, attr string) (err error)
	if fs.FollowSymlinks(ctx) {
		op = unix.Removexattr
	} else {
		op = unix.Lremovexattr
	}

	p := path.Join(fsys.root.Name(), name)
	return op(p, attr)
}
