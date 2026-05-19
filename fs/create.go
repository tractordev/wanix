package fs

import "os"

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

	if rfsys, rname, err := ResolveTo[OpenFileFS](fsys, ctx, name); err == nil {
		return rfsys.OpenFile(rname, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0o666)
	}

	return nil, opErr(fsys, name, "create", err)
}
