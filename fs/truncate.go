package fs

import (
	"fmt"
	"path"
	"reflect"
)

type TruncateFS interface {
	FS
	Truncate(name string, size int64) error
}

func Truncate(fsys FS, name string, size int64) error {
	if t, ok := fsys.(TruncateFS); ok {
		return t.Truncate(name, size)
	}

	if path.Dir(name) != "." {
		parent, err := Sub(fsys, path.Dir(name))
		if err != nil {
			return err
		}
		if subfs, ok := parent.(*SubdirFS); ok && reflect.DeepEqual(subfs.Fsys, fsys) {
			// if parent is a SubdirFS of our fsys, we manually
			// call Truncate to avoid infinite recursion
			full, err := subfs.fullName("truncate", path.Base(name))
			if err != nil {
				return err
			}
			if m, ok := subfs.Fsys.(TruncateFS); ok {
				return m.Truncate(full, size)
			}
			return fmt.Errorf("%w on %T: Truncate %s", ErrNotSupported, subfs.Fsys, full)
		}
		return Truncate(parent, path.Base(name), size)
	}

	b, err := ReadFile(fsys, name)
	if err != nil {
		return err
	}

	return WriteFile(fsys, name, b[:size], 0)
}
