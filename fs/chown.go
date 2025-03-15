package fs

type ChownFS interface {
	FS
	Chown(name string, uid, gid int) error
}

// Chown changes the numeric uid and gid of the named file if supported.
func Chown(fsys FS, name string, uid, gid int) error {
	if c, ok := fsys.(ChownFS); ok {
		return c.Chown(name, uid, gid)
	}

	rfsys, rname, err := ResolveTo[ChownFS](fsys, ContextFor(fsys), name)
	if err == nil {
		return rfsys.Chown(rname, uid, gid)
	}
	return opErr(fsys, name, "chown", err)
}
