//go:build wasm || tinygo

package localfs

import (
	"context"
	"os"

	"tractor.dev/wanix/fs"
)

func newRoot(dir string, fsys *FS) (*FS, error) {
	if dir != "/" {
		panic("platform only supports root directory")
	}
	fsys.create = func(name string) (fs.File, error) {
		return os.Create(name)
	}
	fsys.openContext = func(ctx context.Context, name string) (fs.File, error) {
		return os.Open(name)
	}
	fsys.openFile = func(name string, flag int, perm fs.FileMode) (fs.File, error) {
		return os.OpenFile(name, flag, perm)
	}
	fsys.stat = os.Stat
	fsys.lstat = os.Lstat
	fsys.mkdir = os.Mkdir
	fsys.mkdirAll = os.MkdirAll
	fsys.remove = os.Remove
	fsys.removeAll = os.RemoveAll
	fsys.rename = os.Rename
	fsys.chmod = os.Chmod
	fsys.chown = os.Chown
	fsys.chtimes = os.Chtimes
	fsys.symlink = os.Symlink
	fsys.readlink = os.Readlink
	return fsys, nil
}
