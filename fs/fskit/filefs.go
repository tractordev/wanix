package fskit

import (
	"context"
	iofs "io/fs"

	"tractor.dev/wanix/fs"
)

// FileFS wraps a single fs.File as an fs.FS such that opening
// "." returns that file (not a directory).
func FileFS(file fs.File, name string) fs.FS {
	if name == "" {
		file = &renamedFile{File: file, name: name}
	}
	return &fileFS{File: file}
}

type fileFS struct {
	fs.File
}

func (f *fileFS) Open(name string) (iofs.File, error) {
	return f.OpenContext(context.Background(), name)
}

func (f *fileFS) OpenContext(ctx context.Context, name string) (iofs.File, error) {
	if name == "." {
		return f.File, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}
