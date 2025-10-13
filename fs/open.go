package fs

import (
	"context"
)

type OpenContextFS interface {
	FS
	OpenContext(ctx context.Context, name string) (File, error)
}

// OpenContext is a helper that opens a file with the given context and name
// falling back to Open if context is not supported.
// TODO: change to OpenContext(FS, Context, string)
func OpenContext(ctx context.Context, fsys FS, name string) (File, error) {
	ctx = WithOrigin(ctx, fsys, name, "open")

	// _, fullname, _ := Origin(ctx)
	// log.Println("fs.open:", fullname, reflect.TypeOf(fsys), name)

	if o, ok := fsys.(OpenContextFS); ok {
		return o.OpenContext(ctx, name)
	}

	rfsys, rname, err := ResolveTo[OpenContextFS](fsys, ctx, name)
	if err == nil {
		return rfsys.OpenContext(ctx, rname)
	}

	return fsys.Open(name)
}
