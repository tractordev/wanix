package fs

import (
	"os"
	"time"
)

type OpenFileFS interface {
	FS
	OpenFile(name string, flag int, perm FileMode) (File, error)
}

// OpenFile is a helper that opens a file with the given flag and permissions if supported.
func OpenFile(fsys FS, name string, flag int, perm FileMode) (File, error) {
	if o, ok := fsys.(OpenFileFS); ok {
		return o.OpenFile(name, flag, perm)
	}

	// Handle different file modes
	if flag&os.O_CREATE != 0 {
		return Create(fsys, name)
	}

	// just fall back to Open
	return fsys.Open(name)
}

type ChmodFS interface {
	FS
	Chmod(name string, mode FileMode) error
}

// Chmod changes the mode of the named file if supported.
func Chmod(fsys FS, name string, mode FileMode) error {
	if c, ok := fsys.(ChmodFS); ok {
		return c.Chmod(name, mode)
	}
	return ErrNotSupported
}

type ChownFS interface {
	FS
	Chown(name string, uid, gid int) error
}

// Chown changes the numeric uid and gid of the named file if supported.
func Chown(fsys FS, name string, uid, gid int) error {
	if c, ok := fsys.(ChownFS); ok {
		return c.Chown(name, uid, gid)
	}
	return ErrNotSupported
}

type ChtimesFS interface {
	FS
	Chtimes(name string, atime time.Time, mtime time.Time) error
}

// Chtimes changes the access and modification times of the named file if supported.
func Chtimes(fsys FS, name string, atime time.Time, mtime time.Time) error {
	if c, ok := fsys.(ChtimesFS); ok {
		return c.Chtimes(name, atime, mtime)
	}
	return ErrNotSupported
}

type RenameFS interface {
	FS
	Rename(oldname, newname string) error
}

// Rename renames (moves) oldname to newname if supported.
func Rename(fsys FS, oldname, newname string) error {
	if r, ok := fsys.(RenameFS); ok {
		return r.Rename(oldname, newname)
	}
	return ErrNotSupported
}
