package fs

type ChmodFS interface {
	FS
	Chmod(name string, mode FileMode) error
}

// Chmod changes the mode of the named file if supported.
func Chmod(fsys FS, name string, mode FileMode) error {
	if c, ok := fsys.(ChmodFS); ok {
		return c.Chmod(name, mode)
	}

	rfsys, rname, err := ResolveAs[ChmodFS](fsys, name)
	if err == nil {
		return rfsys.Chmod(rname, mode)
	}
	return opErr(fsys, name, "chmod", err)
}
