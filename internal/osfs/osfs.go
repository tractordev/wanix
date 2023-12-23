// TODO: move into toolkit-go
package osfs

import (
	"os"
	"path/filepath"
	"time"

	"tractor.dev/toolkit-go/engine/fs"
)

type FS struct{}

func New() *FS {
	return &FS{}
}

func (FS) Create(name string) (fs.File, error) {
	f, e := os.Create(fsToUnixPath(name))
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (FS) Mkdir(name string, perm fs.FileMode) error {
	return os.Mkdir(fsToUnixPath(name), perm)
}

func (FS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (FS) Open(name string) (fs.File, error) {
	f, e := os.Open(fsToUnixPath(name))
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	f, e := os.OpenFile(fsToUnixPath(name), flag, perm)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (FS) Remove(name string) error {
	return os.Remove(fsToUnixPath(name))
}

func (FS) RemoveAll(path string) error {
	return os.RemoveAll(fsToUnixPath(path))
}

func (FS) Rename(oldname, newname string) error {
	return os.Rename(fsToUnixPath(oldname), fsToUnixPath(newname))
}

func (FS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(fsToUnixPath(name))
}

func (FS) Chmod(name string, mode fs.FileMode) error {
	return os.Chmod(fsToUnixPath(name), mode)
}

func (FS) Chown(name string, uid, gid int) error {
	return os.Chown(fsToUnixPath(name), uid, gid)
}

func (FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return os.Chtimes(fsToUnixPath(name), atime, mtime)
}

// Converts an `io/fs` path to a Unix path.
// Assumes path is absolute already
func fsToUnixPath(path string) string {
	if !filepath.IsAbs(path) {
		path = "/" + path
	}
	return filepath.Clean(path)
}
