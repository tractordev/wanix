package fskit

import "tractor.dev/wanix/fs"

type namedFS struct {
	fs.FS
	name string
}

func NamedFS(fsys fs.FS, name string) fs.FS {
	return &namedFS{FS: fsys, name: name}
}

func (fsys *namedFS) Open(name string) (fs.File, error) {
	if name == "." {
		f, err := fsys.FS.Open(".")
		if err != nil {
			return nil, err
		}
		if isDir, _ := fs.IsDir(fsys.FS, "."); isDir {
			return &renamedDir{File: f, name: fsys.name}, nil
		} else {
			return &renamedFile{File: f, name: fsys.name}, nil
		}
	}
	return fsys.FS.Open(name)
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
	return Node(fi, f.name), nil
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
	return Node(fi, d.name), nil
}

func (d *renamedDir) ReadDir(count int) ([]fs.DirEntry, error) {
	if rd, ok := d.File.(fs.ReadDirFile); ok {
		return rd.ReadDir(count)
	}
	return nil, &fs.PathError{Op: "readdir", Path: d.name, Err: fs.ErrInvalid}
}
