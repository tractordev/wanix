package fs

import (
	"errors"
	"fmt"
	"path"
	"reflect"
)

type MkdirFS interface {
	FS
	Mkdir(name string, perm FileMode) error
}

// Mkdir creates a directory with the given permissions if supported.
func Mkdir(fsys FS, name string, perm FileMode) error {
	if m, ok := fsys.(MkdirFS); ok {
		return m.Mkdir(name, perm)
	}

	if path.Dir(name) != "." {
		parent, err := Sub(fsys, path.Dir(name))
		if err != nil {
			return err
		}
		if subfs, ok := parent.(*SubdirFS); ok && reflect.DeepEqual(subfs.Fsys, fsys) {
			// if parent is a SubdirFS of our fsys, we manually
			// call Mkdir to avoid infinite recursion
			full, err := subfs.fullName("mkdir", path.Base(name))
			if err != nil {
				return err
			}
			if m, ok := subfs.Fsys.(MkdirFS); ok {
				return m.Mkdir(full, perm)
			}
			return fmt.Errorf("%w on %T: Mkdir %s", ErrNotSupported, subfs.Fsys, full)
		}
		return Mkdir(parent, path.Base(name), perm)
	}

	return fmt.Errorf("%w on %T: Mkdir %s", ErrNotSupported, fsys, name)
}

type MkdirAllFS interface {
	FS
	MkdirAll(path string, perm FileMode) error
}

// MkdirAll creates a directory and any necessary parents with the given permissions if supported.
func MkdirAll(fsys FS, name string, perm FileMode) error {
	if m, ok := fsys.(MkdirAllFS); ok {
		return m.MkdirAll(name, perm)
	}

	err := Mkdir(fsys, name, perm)
	if !errors.Is(err, ErrNotExist) {
		return err
	}

	// parent doesn't exist, make parent dirs and try again
	if path.Dir(name) != "." {
		if err := MkdirAll(fsys, path.Dir(name), perm); err != nil {
			return err
		}
		return Mkdir(fsys, name, perm)
	}

	return fmt.Errorf("%w on %T: MkdirAll %s", ErrNotSupported, fsys, name)
}
