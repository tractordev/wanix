package fs

import (
	"path"
)

type RenameFS interface {
	FS
	Rename(oldname, newname string) error
}

// Rename renames (moves) oldname to newname if supported.
func Rename(fsys FS, oldname, newname string) error {
	if r, ok := fsys.(RenameFS); ok {
		return r.Rename(oldname, newname)
	}

	if exists, err := Exists(fsys, oldname); err != nil || !exists {
		return opErr(fsys, newname, "rename", ErrNotExist)
	}

	oldfsys, oldrname, err := ResolveTo[RenameFS](fsys, ContextFor(fsys), oldname)
	if err != nil {
		return opErr(fsys, newname, "rename", err)
	}

	newfsys, newrdir, err := ResolveTo[RenameFS](fsys, ContextFor(fsys), path.Dir(newname))
	if err != nil {
		return opErr(fsys, newname, "rename", err)
	}

	if Equal(oldfsys, newfsys) {
		return oldfsys.Rename(oldrname, path.Join(newrdir, path.Base(newname)))
	}

	// fallback to copy and remove across filesystems

	if err := CopyFS(oldfsys, oldrname, newfsys, path.Join(newrdir, path.Base(newname))); err != nil {
		return opErr(fsys, newname, "rename", err)
	}

	if err := RemoveAll(oldfsys, oldrname); err != nil {
		return opErr(fsys, newname, "rename", err)
	}

	return nil
}
