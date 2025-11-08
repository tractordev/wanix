package fs

import (
	"errors"
	"path"
)

type RemoveFS interface {
	FS
	Remove(name string) error
}

// Remove removes the named file or empty directory if supported.
func Remove(fsys FS, name string) error {
	if r, ok := fsys.(RemoveFS); ok {
		return r.Remove(name)
	}

	rfsys, rname, err := ResolveTo[RemoveFS](fsys, ContextFor(fsys), name)
	if err == nil {
		return rfsys.Remove(rname)
	}

	// for non-existent files, return ErrNotExist
	// before returning ErrNotSupported
	_, err = Stat(fsys, name)
	if err != nil {
		return err
	}

	return opErr(fsys, name, "remove", err)
}

type RemoveAllFS interface {
	FS
	RemoveAll(path string) error
}

// RemoveAll removes path name and any children it contains if supported.
func RemoveAll(fsys FS, name string) error {
	if r, ok := fsys.(RemoveAllFS); ok {
		return r.RemoveAll(name)
	}

	rfsys, rname, err := ResolveTo[RemoveAllFS](fsys, ContextFor(fsys), name)
	if err == nil {
		return rfsys.RemoveAll(rname)
	}
	if !errors.Is(err, ErrNotSupported) {
		return opErr(fsys, name, "removeall", err)
	}

	err = Remove(fsys, name)
	if !errors.Is(err, ErrNotEmpty) {
		return err
	}

	// not empty, remove all children, then remove self

	children, err := ReadDir(fsys, name)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := RemoveAll(fsys, path.Join(name, child.Name())); err != nil {
			return err
		}
	}

	return Remove(fsys, name)
}
