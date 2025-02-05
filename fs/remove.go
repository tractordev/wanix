package fs

import (
	"errors"
	"fmt"
	"path"
	"reflect"
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

	if path.Dir(name) != "." {
		parent, err := Sub(fsys, path.Dir(name))
		if err != nil {
			return err
		}
		if subfs, ok := parent.(*SubdirFS); ok && reflect.DeepEqual(subfs.Fsys, fsys) {
			// if parent is a SubdirFS of our fsys, we manually
			// call Remove to avoid infinite recursion
			full, err := subfs.fullName("remove", path.Base(name))
			if err != nil {
				return err
			}
			if r, ok := subfs.Fsys.(RemoveFS); ok {
				return r.Remove(full)
			}
			return fmt.Errorf("%w on %T: Remove %s", ErrNotSupported, subfs.Fsys, full)
		}
		return Remove(parent, path.Base(name))
	}

	return fmt.Errorf("%w on %T: Remove %s", ErrNotSupported, fsys, name)
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

	err := Remove(fsys, name)
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
