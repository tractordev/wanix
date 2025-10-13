// tarfs implements a read-only in-memory representation of a tar archive
package tarfs

import (
	"archive/tar"
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

var Separator = "/"

type Reader struct {
	files map[string]map[string]*File
}

func splitpath(name string) (dir, file string) {
	name = filepath.ToSlash(name)
	if len(name) == 0 || name[0] != '/' {
		name = "/" + name
	}
	name = filepath.Clean(name)
	dir, file = filepath.Split(name)
	dir = filepath.Clean(dir)
	return
}

func From(t *tar.Reader) *Reader {
	fsys := &Reader{files: make(map[string]map[string]*File)}
	for {
		hdr, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}

		d, f := splitpath(hdr.Name)
		if _, ok := fsys.files[d]; !ok {
			fsys.files[d] = make(map[string]*File)
		}

		var buf bytes.Buffer
		size, err := buf.ReadFrom(t)
		if err != nil {
			panic("tarfs: reading from tar:" + err.Error())
		}

		if size != hdr.Size {
			panic("tarfs: size mismatch")
		}

		file := &File{
			h:    hdr,
			data: bytes.NewReader(buf.Bytes()),
			fs:   fsys,
		}
		fsys.files[d][f] = file

	}

	if fsys.files[Separator] == nil {
		fsys.files[Separator] = make(map[string]*File)
	}
	// Add a pseudoroot
	fsys.files[Separator][""] = &File{
		h: &tar.Header{
			Name:     Separator,
			Typeflag: tar.TypeDir,
			Size:     0,
		},
		data: bytes.NewReader(nil),
		fs:   fsys,
	}

	return fsys
}

func (fsys *Reader) Open(name string) (fs.File, error) {
	d, f := splitpath(name)
	if _, ok := fsys.files[d]; !ok {
		return nil, &os.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	file, ok := fsys.files[d][f]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	nf := *file
	br := *nf.data
	nf.data = &br

	return &nf, nil
}

func (fsys *Reader) Stat(name string) (fs.FileInfo, error) {
	d, f := splitpath(name)
	if _, ok := fsys.files[d]; !ok {
		return nil, &os.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}

	file, ok := fsys.files[d][f]
	if !ok {
		return nil, &os.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}

	return file.h.FileInfo(), nil
}

func (fsys *Reader) Readlink(name string) (string, error) {
	d, f := splitpath(name)
	if _, ok := fsys.files[d]; !ok {
		return "", &os.PathError{Op: "readlink", Path: name, Err: fs.ErrNotExist}
	}

	file, ok := fsys.files[d][f]
	if !ok {
		return "", &os.PathError{Op: "readlink", Path: name, Err: fs.ErrNotExist}
	}

	if file.h.Typeflag != tar.TypeSymlink {
		return "", &os.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}

	return file.h.Linkname, nil
}

type File struct {
	h      *tar.Header
	data   *bytes.Reader
	closed bool
	fs     *Reader
}

func (f *File) Close() error {
	if f.closed {
		return fs.ErrClosed
	}

	f.closed = true
	f.h = nil
	f.data = nil
	f.fs = nil

	return nil
}

func (f *File) Read(p []byte) (n int, err error) {
	if f.closed {
		return 0, fs.ErrClosed
	}

	if f.h.Typeflag == tar.TypeDir {
		return 0, fs.ErrInvalid
	}

	return f.data.Read(p)
}

func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	if f.closed {
		return 0, fs.ErrClosed
	}

	if f.h.Typeflag == tar.TypeDir {
		return 0, fs.ErrInvalid
	}

	return f.data.ReadAt(p, off)
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f.closed {
		return 0, fs.ErrClosed
	}

	if f.h.Typeflag == tar.TypeDir {
		return 0, fs.ErrInvalid
	}

	return f.data.Seek(offset, whence)
}

func (f *File) Name() string {
	return filepath.Join(splitpath(f.h.Name))
}

func (f *File) getDirectoryNames() ([]string, error) {
	d, ok := f.fs.files[f.Name()]
	if !ok {
		return nil, nil
		//return nil, &os.PathError{Op: "readdir", Path: f.Name(), Err: fs.ErrNotExist}
	}

	var names []string
	for n := range d {
		names = append(names, n)
	}
	sort.Strings(names)

	return names, nil
}

func (f *File) ReadDir(count int) ([]fs.DirEntry, error) {
	if f.closed {
		return nil, fs.ErrClosed
	}

	if !f.h.FileInfo().IsDir() {
		return nil, fs.ErrInvalid
	}

	names, err := f.getDirectoryNames()
	if err != nil {
		return nil, err
	}

	d := f.fs.files[f.Name()]
	var fi []fs.DirEntry
	for _, n := range names {
		if n == "" {
			continue
		}

		f := d[n]
		fi = append(fi, &dirEntry{f.h.FileInfo()})
		if count > 0 && len(fi) >= count {
			break
		}
	}

	return fi, nil
}

func (f *File) Stat() (fs.FileInfo, error) { return f.h.FileInfo(), nil }

type dirEntry struct {
	fs.FileInfo
}

func (d *dirEntry) Info() (fs.FileInfo, error) {
	return d.FileInfo, nil
}

func (d *dirEntry) Type() fs.FileMode {
	return d.FileInfo.Mode()
}
