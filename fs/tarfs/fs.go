// tarfs implements a read-only in-memory representation of a tar archive
package tarfs

import (
	"archive/tar"
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

var Separator = "/"

type FS struct {
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

func New(t *tar.Reader) *FS {
	fsys := &FS{files: make(map[string]map[string]*File)}
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

func (fsys *FS) Open(name string) (fs.File, error) {
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

func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
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
