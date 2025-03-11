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
func OpenContext(ctx context.Context, fsys FS, name string) (File, error) {
	ctx = WithOrigin(ctx, fsys)
	ctx = WithFilepath(ctx, name)

	// _, fullname, _ := Origin(ctx)
	// log.Println("fs.open:", fullname, reflect.TypeOf(fsys), name)

	if o, ok := fsys.(OpenContextFS); ok {
		// log.Println("fs.opencontext: fsys->", name, reflect.TypeOf(fsys))
		f, e := o.OpenContext(ctx, name)
		// log.Println("fs.opencontext: fsys<-", name, reflect.TypeOf(fsys), e)
		return f, e
	}

	rfsys, rname, err := ResolveAs[OpenContextFS](fsys, name)
	if err == nil {
		// log.Println("fs.opencontext: rfsys", rname, reflect.TypeOf(rfsys))
		return rfsys.OpenContext(ctx, rname)
	}

	// log.Println("fs.opencontext: open", name, reflect.TypeOf(fsys))
	return fsys.Open(name)
}
