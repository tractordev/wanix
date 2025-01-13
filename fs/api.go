package fs

import (
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
	// TODO: implement derived OpenFile using Open
	return nil, ErrNotSupported
}

type CreateFS interface {
	FS
	Create(name string) (File, error)
}

// Create creates or truncates the named file if supported.
func Create(fsys FS, name string) (File, error) {
	if c, ok := fsys.(CreateFS); ok {
		return c.Create(name)
	}
	// TODO: implement derived Create using OpenFile
	return nil, ErrNotSupported
}

type MkdirFS interface {
	FS
	Mkdir(name string, perm FileMode) error
}

// Mkdir creates a directory with the given permissions if supported.
func Mkdir(fsys FS, name string, perm FileMode) error {
	if m, ok := fsys.(MkdirFS); ok {
		return m.Mkdir(name, perm)
	}
	return ErrNotSupported
}

type MkdirAllFS interface {
	FS
	MkdirAll(path string, perm FileMode) error
}

// MkdirAll creates a directory and any necessary parents with the given permissions if supported.
func MkdirAll(fsys FS, path string, perm FileMode) error {
	if m, ok := fsys.(MkdirAllFS); ok {
		return m.MkdirAll(path, perm)
	}
	// TODO: implement derived MkdirAll using Mkdir
	return ErrNotSupported
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

type RemoveFS interface {
	FS
	Remove(name string) error
}

// Remove removes the named file or empty directory if supported.
func Remove(fsys FS, name string) error {
	if r, ok := fsys.(RemoveFS); ok {
		return r.Remove(name)
	}
	return ErrNotSupported
}

type RemoveAllFS interface {
	FS
	RemoveAll(path string) error
}

// RemoveAll removes path and any children it contains if supported.
func RemoveAll(fsys FS, path string) error {
	if r, ok := fsys.(RemoveAllFS); ok {
		return r.RemoveAll(path)
	}
	// TODO: implement derived RemoveAll using Remove
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
