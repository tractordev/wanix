package fskit

import (
	"context"
	iofs "io/fs"

	"tractor.dev/wanix/fs"
)

// FileFS wraps a single fs.File as an fs.FS such that opening
// "." returns that file (not a directory). This matches places in
// the project that expect the FS root to be the file itself.
func FileFS(file fs.File, name string) fs.FS {
	return &fileFS{file: file, name: name}
}

type fileFS struct {
	file fs.File
	name string
}

var _ fs.FS = (*fileFS)(nil)
var _ fs.OpenContextFS = (*fileFS)(nil)
var _ fs.StatFS = (*fileFS)(nil)

func (f *fileFS) Open(name string) (iofs.File, error) {
	return f.OpenContext(context.Background(), name)
}

func (f *fileFS) Stat(name string) (iofs.FileInfo, error) {
	if name == "." {
		fi, err := f.file.Stat()
		if err != nil {
			return Entry(f.name, 0644), nil
		}
		return RawNode(fi, f.name), nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

func (f *fileFS) OpenContext(ctx context.Context, name string) (iofs.File, error) {
	// Only "." is supported and maps to the file itself
	if name == "." {
		return &renamedFile{File: f.file, name: f.name}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}
