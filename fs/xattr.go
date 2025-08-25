package fs

import "context"

type XattrFS interface {
	FS
	SetXattr(ctx context.Context, name string, attr string, data []byte, flags int) error
	GetXattr(ctx context.Context, name string, attr string) ([]byte, error)
	ListXattrs(ctx context.Context, name string) ([]string, error)
	RemoveXattr(ctx context.Context, name string, attr string) error
}

func SetXattr(ctx context.Context, fsys FS, name string, attr string, data []byte, flags int) error {
	if c, ok := fsys.(XattrFS); ok {
		return c.SetXattr(ctx, name, attr, data, flags)
	}

	rfsys, rname, err := ResolveTo[XattrFS](fsys, ContextFor(fsys), name)
	if err == nil {
		return rfsys.SetXattr(ctx, rname, attr, data, flags)
	}

	return opErr(fsys, name, "setxattr", err)
}

func GetXattr(ctx context.Context, fsys FS, name string, attr string) ([]byte, error) {
	if c, ok := fsys.(XattrFS); ok {
		return c.GetXattr(ctx, name, attr)
	}

	rfsys, rname, err := ResolveTo[XattrFS](fsys, ContextFor(fsys), name)
	if err == nil {
		return rfsys.GetXattr(ctx, rname, attr)
	}

	return nil, opErr(fsys, name, "getxattr", err)
}

func ListXattrs(ctx context.Context, fsys FS, name string) ([]string, error) {
	if c, ok := fsys.(XattrFS); ok {
		return c.ListXattrs(ctx, name)
	}

	rfsys, rname, err := ResolveTo[XattrFS](fsys, ContextFor(fsys), name)
	if err == nil {
		return rfsys.ListXattrs(ctx, rname)
	}

	return nil, opErr(fsys, name, "listxattrs", err)
}

func RemoveXattr(ctx context.Context, fsys FS, name string, attr string) error {
	if c, ok := fsys.(XattrFS); ok {
		return c.RemoveXattr(ctx, name, attr)
	}

	rfsys, rname, err := ResolveTo[XattrFS](fsys, ContextFor(fsys), name)
	if err == nil {
		return rfsys.RemoveXattr(ctx, rname, attr)
	}

	return opErr(fsys, name, "removexattr", err)
}
