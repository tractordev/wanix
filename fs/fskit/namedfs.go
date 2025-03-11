package fskit

import (
	"context"

	"tractor.dev/wanix/fs"
)

type namedFS struct {
	fs.FS
	name string
}

func NamedFS(fsys fs.FS, name string) fs.FS {
	return &namedFS{FS: fsys, name: name}
}

func (fsys *namedFS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

func (fsys *namedFS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if name == "." {
		f, err := fs.OpenContext(ctx, fsys.FS, ".")
		if err != nil {
			return nil, err
		}
		fi, err := f.Stat()
		if err != nil {
			return nil, err
		}
		if fi.IsDir() {
			return &renamedDir{File: f, name: fsys.name}, nil
		} else {
			return &renamedFile{File: f, name: fsys.name}, nil
		}
	}
	return fs.OpenContext(ctx, fsys.FS, name)
}

type renamedDir struct {
	fs.File
	name string
}

func (d *renamedDir) Stat() (fs.FileInfo, error) {
	fi, err := d.File.Stat()
	if err != nil {
		return nil, err
	}
	return RawNode(fi, d.name), nil
}

func (d *renamedDir) ReadDir(count int) ([]fs.DirEntry, error) {
	if rd, ok := d.File.(fs.ReadDirFile); ok {
		return rd.ReadDir(count)
	}
	return nil, &fs.PathError{Op: "readdir", Path: d.name, Err: fs.ErrInvalid}
}

type renamedFile struct {
	fs.File
	name string
}

func (f *renamedFile) Stat() (fs.FileInfo, error) {
	fi, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return RawNode(fi, f.name), nil
}

// we also have to implement any other interfaces that we want to expose
// from the underlying file implementation

func (f *renamedFile) Seek(offset int64, whence int) (int64, error) {
	return fs.Seek(f.File, offset, whence)
}

func (f *renamedFile) ReadAt(b []byte, offset int64) (int, error) {
	return fs.ReadAt(f.File, b, offset)
}

func (f *renamedFile) WriteAt(b []byte, offset int64) (int, error) {
	return fs.WriteAt(f.File, b, offset)
}

func (f *renamedFile) Write(b []byte) (int, error) {
	return fs.Write(f.File, b)
}
