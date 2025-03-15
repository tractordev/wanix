package fs

import (
	"errors"
)

type TruncateFS interface {
	FS
	Truncate(name string, size int64) error
}

func Truncate(fsys FS, name string, size int64) error {
	if t, ok := fsys.(TruncateFS); ok {
		return t.Truncate(name, size)
	}

	rfsys, rname, err := ResolveTo[TruncateFS](fsys, ContextFor(fsys), name)
	if err == nil {
		return rfsys.Truncate(rname, size)
	}
	if !errors.Is(err, ErrNotSupported) {
		return opErr(fsys, name, "truncate", err)
	}

	b, err := ReadFile(fsys, name)
	if err != nil {
		return err
	}

	return WriteFile(fsys, name, b[:size], 0)
}
