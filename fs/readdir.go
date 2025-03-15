package fs

import (
	"context"
	"errors"
	"slices"
	"strings"
)

type ReadDirContextFS interface {
	FS
	ReadDirContext(ctx context.Context, name string) ([]DirEntry, error)
}

func ReadDirContext(ctx context.Context, fsys FS, name string) ([]DirEntry, error) {
	ctx = WithOrigin(ctx, fsys, name, "readdir")
	ctx = WithReadOnly(ctx)

	if fsys, ok := fsys.(ReadDirContextFS); ok {
		return fsys.ReadDirContext(ctx, name)
	}

	rfsys, rname, err := ResolveTo[ReadDirContextFS](fsys, ctx, name)
	if err == nil {
		return rfsys.ReadDirContext(ctx, rname)
	}
	if !errors.Is(err, ErrNotSupported) {
		return nil, opErr(fsys, name, "readdir", err)
	}

	file, err := OpenContext(ctx, fsys, name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dir, ok := file.(ReadDirFile)
	if !ok {
		// todo: use opErr?
		return nil, &PathError{Op: "readdir", Path: name, Err: errors.New("not implemented")}
	}

	list, err := dir.ReadDir(-1)
	slices.SortFunc(list, func(a, b DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return list, err
}
