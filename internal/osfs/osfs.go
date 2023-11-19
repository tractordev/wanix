// TODO: move into toolkit-go
package osfs

import (
	"os"
	"time"

	"tractor.dev/toolkit-go/engine/fs"
)

type FS struct{}

func New() *FS {
	return &FS{}
}

func (FS) Create(name string) (fs.File, error) {
	f, e := os.Create(name)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (FS) Mkdir(name string, perm fs.FileMode) error {
	return os.Mkdir(name, perm)
}

func (FS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (FS) Open(name string) (fs.File, error) {
	f, e := os.Open(name)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	f, e := os.OpenFile(name, flag, perm)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (FS) Remove(name string) error {
	return os.Remove(name)
}

func (FS) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (FS) Rename(oldname, newname string) error {
	return os.Rename(oldname, newname)
}

func (FS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

func (FS) Chmod(name string, mode fs.FileMode) error {
	return os.Chmod(name, mode)
}

func (FS) Chown(name string, uid, gid int) error {
	return os.Chown(name, uid, gid)
}

func (FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return os.Chtimes(name, atime, mtime)
}
