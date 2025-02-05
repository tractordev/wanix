package fs

import (
	"fmt"
	"path"
	"reflect"
)

type CreateFS interface {
	FS
	Create(name string) (File, error)
}

// Create creates or truncates the named file if supported.
func Create(fsys FS, name string) (File, error) {
	if c, ok := fsys.(CreateFS); ok {
		return c.Create(name)
	}

	if path.Dir(name) != "." {
		parent, err := Sub(fsys, path.Dir(name))
		if err != nil {
			return nil, err
		}
		if subfs, ok := parent.(*SubdirFS); ok && reflect.DeepEqual(subfs.Fsys, fsys) {
			// if parent is a SubdirFS of our fsys, we manually
			// call Create to avoid infinite recursion
			full, err := subfs.fullName("create", path.Base(name))
			if err != nil {
				return nil, err
			}
			if c, ok := subfs.Fsys.(CreateFS); ok {
				return c.Create(full)
			}
			return nil, fmt.Errorf("%w on %T: Create %s", ErrNotSupported, subfs.Fsys, full)
		}
		return Create(parent, path.Base(name))
	}

	// TODO: implement derived Create using OpenFile

	return nil, fmt.Errorf("%w on %T: Create %s", ErrNotSupported, fsys, name)
}
