package fs

import (
	"errors"
	"path"
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

	ctx := WithOrigin(ContextFor(fsys), fsys, name, "mkdir")
	rfsys, rname, err := ResolveTo[MkdirFS](fsys, ctx, name) // path.Dir(name))
	if err == nil {
		return rfsys.Mkdir(rname, perm) //path.Join(rdir, path.Base(name)), perm)
	}
	return opErr(fsys, name, "mkdir", err)
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

	rfsys, rname, err := ResolveTo[MkdirAllFS](fsys, ContextFor(fsys), name) // path.Dir(name))
	if err == nil {
		return rfsys.MkdirAll(rname, perm) //path.Join(rdir, path.Base(name)), perm)
	}
	if !errors.Is(err, ErrNotSupported) {
		return opErr(fsys, name, "mkdirall", err)
	}

	err = Mkdir(fsys, name, perm)
	if err == nil || errors.Is(err, ErrExist) {
		return nil
	}

	if !errors.Is(err, ErrNotExist) || path.Dir(name) == "." {
		return opErr(fsys, name, "mkdirall", err)
	}

	// parent doesn't exist, make parent dirs and try again
	if err := MkdirAll(fsys, path.Dir(name), perm); err != nil {
		return err
	}
	return Mkdir(fsys, name, perm)
}
