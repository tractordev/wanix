package fs

import (
	"context"
	"io/fs"
	"time"
)

// DefaultFS wraps an fs.FS and implements all extended filesystem interfaces
// (MkdirFS, RemoveFS, RenameFS, etc.) by delegating to the corresponding
// package-level functions.
//
// This is useful when embedding a filesystem in a new struct that only needs
// to override specific methods. Normally, if you embed an fs.FS interface in
// a struct, type assertions for extended interfaces will fail even if the
// underlying filesystem implements them. By wrapping with DefaultFS first,
// the wrapper struct gains implementations of all extended interfaces that
// properly delegate to the wrapped filesystem.
//
// Example:
//
//	type myFS struct {
//	    *fs.DefaultFS
//	}
//
//	func (m *myFS) Open(name string) (fs.File, error) {
//	    // custom Open implementation
//	}
//
//	// Create wrapper - Mkdir, Remove, etc. will reach originalFS
//	wrapped := &myFS{DefaultFS: fs.NewDefault(originalFS)}
type DefaultFS struct {
	fs.FS
}

// NewDefault wraps an fs.FS with DefaultFS to enable interface passthrough.
func NewDefault(fsys fs.FS) *DefaultFS {
	return &DefaultFS{
		FS: fsys,
	}
}

func (f *DefaultFS) Stat(name string) (FileInfo, error) {
	return Stat(f.FS, name)
}

func (f *DefaultFS) Lstat(name string) (FileInfo, error) {
	return Lstat(f.FS, name)
}

func (f *DefaultFS) StatContext(ctx context.Context, name string) (FileInfo, error) {
	return StatContext(ctx, f.FS, name)
}

func (f *DefaultFS) LstatContext(ctx context.Context, name string) (FileInfo, error) {
	return LstatContext(ctx, f.FS, name)
}

func (f *DefaultFS) OpenContext(ctx context.Context, name string) (File, error) {
	return OpenContext(ctx, f.FS, name)
}

func (f *DefaultFS) OpenFile(name string, flag int, perm FileMode) (File, error) {
	return OpenFile(f.FS, name, flag, perm)
}

func (f *DefaultFS) Create(name string) (File, error) {
	return Create(f.FS, name)
}

func (f *DefaultFS) Mkdir(name string, perm FileMode) error {
	return Mkdir(f.FS, name, perm)
}

func (f *DefaultFS) MkdirAll(name string, perm FileMode) error {
	return MkdirAll(f.FS, name, perm)
}

func (f *DefaultFS) Remove(name string) error {
	return Remove(f.FS, name)
}

func (f *DefaultFS) RemoveAll(name string) error {
	return RemoveAll(f.FS, name)
}

func (f *DefaultFS) Rename(oldname, newname string) error {
	return Rename(f.FS, oldname, newname)
}

func (f *DefaultFS) Chmod(name string, mode FileMode) error {
	return Chmod(f.FS, name, mode)
}

func (f *DefaultFS) Chown(name string, uid, gid int) error {
	return Chown(f.FS, name, uid, gid)
}

func (f *DefaultFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return Chtimes(f.FS, name, atime, mtime)
}

func (f *DefaultFS) Truncate(name string, size int64) error {
	return Truncate(f.FS, name, size)
}

func (f *DefaultFS) Symlink(oldname, newname string) error {
	return Symlink(f.FS, oldname, newname)
}

func (f *DefaultFS) Readlink(name string) (string, error) {
	return Readlink(f.FS, name)
}

func (f *DefaultFS) WriteFile(name string, data []byte, perm FileMode) error {
	return WriteFile(f.FS, name, data, perm)
}

func (f *DefaultFS) ReadDirContext(ctx context.Context, name string) ([]DirEntry, error) {
	return ReadDirContext(ctx, f.FS, name)
}

func (f *DefaultFS) Sub(dir string) (FS, error) {
	return Sub(f.FS, dir)
}

func (f *DefaultFS) SetXattr(ctx context.Context, name string, attr string, data []byte, flags int) error {
	return SetXattr(ctx, f.FS, name, attr, data, flags)
}

func (f *DefaultFS) GetXattr(ctx context.Context, name string, attr string) ([]byte, error) {
	return GetXattr(ctx, f.FS, name, attr)
}

func (f *DefaultFS) ListXattrs(ctx context.Context, name string) ([]string, error) {
	return ListXattrs(ctx, f.FS, name)
}

func (f *DefaultFS) RemoveXattr(ctx context.Context, name string, attr string) error {
	return RemoveXattr(ctx, f.FS, name, attr)
}

func (f *DefaultFS) Watch(ctx context.Context, name string, exclude ...string) (<-chan Event, error) {
	return Watch(f.FS, ctx, name, exclude...)
}

// DefaultFile wraps an fs.File and implements extended file interfaces
// (io.Writer, io.ReaderAt, io.WriterAt, io.Seeker, SyncFile) by delegating
// to the corresponding package-level functions.
//
// Similar to DefaultFS, this allows embedding a file in a new struct while
// preserving access to the wrapped file's capabilities through type assertions.
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

func (f DefaultFile) Sync() error {
	return Sync(f.File)
}
