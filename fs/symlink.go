package fs

type SymlinkFS interface {
	FS
	Symlink(oldname, newname string) error
}

func Symlink(fsys FS, oldname, newname string) error {
	if c, ok := fsys.(SymlinkFS); ok {
		return c.Symlink(oldname, newname)
	}

	ctx := WithOrigin(ContextFor(fsys), fsys, newname, "symlink")
	rfsys, rname, err := ResolveTo[SymlinkFS](fsys, ctx, newname) //path.Dir(newname))
	if err == nil {
		return rfsys.Symlink(oldname, rname) //path.Join(rdir, path.Base(newname)))
	}
	return opErr(fsys, newname, "symlink", err)
}
