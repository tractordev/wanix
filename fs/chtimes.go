package fs

import (
	"time"
)

type ChtimesFS interface {
	FS
	Chtimes(name string, atime time.Time, mtime time.Time) error
}

// Chtimes changes the access and modification times of the named file if supported.
func Chtimes(fsys FS, name string, atime time.Time, mtime time.Time) error {
	if c, ok := fsys.(ChtimesFS); ok {
		return c.Chtimes(name, atime, mtime)
	}

	rfsys, rname, err := ResolveAs[ChtimesFS](fsys, name)
	if err == nil {
		return rfsys.Chtimes(rname, atime, mtime)
	}
	return opErr(fsys, name, "chtimes", err)
}
