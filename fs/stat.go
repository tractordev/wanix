package fs

import (
	"context"
	"errors"
)

func Stat(fsys FS, name string) (FileInfo, error) {
	return StatContext(ContextFor(fsys), fsys, name)
}

func Lstat(fsys FS, name string) (FileInfo, error) {
	return StatContext(WithNoFollow(ContextFor(fsys)), fsys, name)
}

type StatContextFS interface {
	FS
	StatContext(ctx context.Context, name string) (FileInfo, error)
}

// TODO: change to StatContext(FS, Context, string)
func StatContext(ctx context.Context, fsys FS, name string) (FileInfo, error) {
	ctx = WithOrigin(ctx, fsys, name, "stat")

	// _, fullname, _ := Origin(ctx)
	// log.Println("fs.statcontext:", name, reflect.TypeOf(fsys), fullname)
	// fmt.Println(string(debug.Stack()))

	if fsys, ok := fsys.(StatContextFS); ok {
		return fsys.StatContext(ctx, name)
	}

	rfsys, rname, err := ResolveTo[StatContextFS](fsys, ctx, name)
	if err == nil {
		return rfsys.StatContext(ctx, rname)
	}
	if !errors.Is(err, ErrNotSupported) {
		return nil, opErr(fsys, name, "stat", err)
	}

	file, err := OpenContext(ctx, fsys, name)
	if err != nil {
		return nil, opErr(fsys, name, "stat", err)
	}
	defer file.Close()
	return file.Stat()
}
