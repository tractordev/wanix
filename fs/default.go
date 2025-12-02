package fs

import "io/fs"

type DefaultFS struct {
	fs.FS
}

func NewDefault(fsys fs.FS) *DefaultFS {
	return &DefaultFS{
		FS: fsys,
	}
}

type DefaultFile struct {
	fs.File
}

func (f DefaultFile) Write(p []byte) (int, error) {
	return Write(f.File, p)
}

func (f DefaultFile) ReadAt(p []byte, off int64) (int, error) {
	return ReadAt(f.File, p, off)
}

func (f DefaultFile) WriteAt(p []byte, off int64) (int, error) {
	return WriteAt(f.File, p, off)
}

func (f DefaultFile) Seek(offset int64, whence int) (int64, error) {
	return Seek(f.File, offset, whence)
}
