//go:build !windows && !openbsd

package localfs

import (
	"context"
	"path"
	"strings"

	"golang.org/x/sys/unix"
	"tractor.dev/wanix/fs"
)

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
