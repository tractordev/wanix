package fs

import (
	"context"
	"errors"
	"slices"
	"strings"
)

type OpenContextFS interface {
	FS
	OpenContext(ctx context.Context, name string) (File, error)
}

// OpenContext is a helper that opens a file with the given context and name
// falling back to Open if context is not supported.
func OpenContext(ctx context.Context, fsys FS, name string) (File, error) {
	if o, ok := fsys.(OpenContextFS); ok {
		return o.OpenContext(ctx, name)
	}
	return fsys.Open(name)
}

func ReadDirContext(ctx context.Context, fsys FS, name string) ([]DirEntry, error) {
	// ReadDirFS doesn't implement context
	// if fsys, ok := fsys.(ReadDirFS); ok {
	// 	return fsys.ReadDir(name)
	// }

	file, err := OpenContext(ctx, fsys, name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dir, ok := file.(ReadDirFile)
	if !ok {
		return nil, &PathError{Op: "readdir", Path: name, Err: errors.New("not implemented")}
	}

	list, err := dir.ReadDir(-1)
	slices.SortFunc(list, func(a, b DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return list, err
}

type StatContextFS interface {
	FS
	StatContext(ctx context.Context, name string) (FileInfo, error)
}

func StatContext(ctx context.Context, fsys FS, name string) (FileInfo, error) {
	if fsys, ok := fsys.(StatContextFS); ok {
		return fsys.StatContext(ctx, name)
	}

	file, err := OpenContext(ctx, fsys, name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}
