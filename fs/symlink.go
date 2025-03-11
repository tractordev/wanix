package fs

type SymlinkFS interface {
	FS
	Symlink(oldname, newname string) error
}

func Symlink(fsys FS, oldname, newname string) error {
	if c, ok := fsys.(SymlinkFS); ok {
		return c.Symlink(oldname, newname)
	}

	rfsys, rname, err := ResolveAs[SymlinkFS](fsys, newname)
	// log.Println("symlink resolve:", reflect.TypeOf(rfsys), rname, err)
	if err == nil {
		return rfsys.Symlink(oldname, rname)
	}
	return opErr(fsys, newname, "symlink", err)
}
