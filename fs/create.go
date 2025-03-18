package fs

type CreateFS interface {
	FS
	Create(name string) (File, error)
}

// Create creates or truncates the named file if supported.
func Create(fsys FS, name string) (File, error) {
	if c, ok := fsys.(CreateFS); ok {
		return c.Create(name)
	}

	ctx := WithOrigin(ContextFor(fsys), fsys, name, "create")
	rfsys, rname, err := ResolveTo[CreateFS](fsys, ctx, name) //path.Dir(name))
	if err == nil {
		return rfsys.Create(rname) //path.Join(rdir, path.Base(name)))
	}

	// TODO: implement derived Create using OpenFile?

	return nil, opErr(fsys, name, "create", err)
}
