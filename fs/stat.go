package fs

import (
	"context"
	"errors"
)

func Stat(fsys FS, name string) (FileInfo, error) {
	return StatContext(context.Background(), fsys, name)
}

type StatContextFS interface {
	FS
	StatContext(ctx context.Context, name string) (FileInfo, error)
}

func StatContext(ctx context.Context, fsys FS, name string) (FileInfo, error) {
	ctx = WithOrigin(ctx, fsys)
	ctx = WithFilepath(ctx, name)

	// _, fullname, _ := Origin(ctx)
	// log.Println("fs.statcontext:", name, reflect.TypeOf(fsys), fullname)
	// fmt.Println(string(debug.Stack()))

	if fsys, ok := fsys.(StatContextFS); ok {
		// log.Println("fs.statcontext: fsys", name)
		return fsys.StatContext(ctx, name)
	}

	rfsys, rname, err := ResolveAs[StatContextFS](fsys, name)
	if err == nil {
		// log.Println("fs.statcontext: rfsys", name)
		return rfsys.StatContext(ctx, rname)
	}
	if !errors.Is(err, ErrNotSupported) {
		// log.Println("fs.statcontext: err", name, err)
		return nil, opErr(fsys, name, "stat", err)
	}

	// log.Println("fs.statcontext: opencontext", name, err)
	file, err := OpenContext(ctx, fsys, name)
	if err != nil {
		return nil, opErr(fsys, name, "stat", err)
	}
	defer file.Close()
	return file.Stat()
}
