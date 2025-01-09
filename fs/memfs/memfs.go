package memfs

import (
	"io"
	"path"
	"slices"
	"strings"
	"time"

	"tractor.dev/wanix/fs"
)

// A FS is a simple in-memory file system for use in tests,
// represented as a map from path names (arguments to Open)
// to information about the files or directories they represent.
//
// The map need not include parent directories for files contained
// in the map; those will be synthesized if needed.
// But a directory can still be included by setting the [MapFile.Mode]'s [fs.ModeDir] bit;
// this may be necessary for detailed control over the directory's [fs.FileInfo]
// or to create an empty directory.
//
// File system operations read directly from the map,
// so that the file system can be changed by editing the map as needed.
// An implication is that file system operations must not run concurrently
// with changes to the map, which would be a race.
// Another implication is that opening or reading a directory requires
// iterating over the entire map, so a FS should typically be used with not more
// than a few hundred entries or directory reads.
type FS map[string]*MapFile

// A MapFile describes a single file in a [FS].
type MapFile struct {
	Data    []byte      // file content
	Mode    fs.FileMode // fs.FileInfo.Mode
	ModTime time.Time   // fs.FileInfo.ModTime
	Sys     any         // fs.FileInfo.Sys
}

var _ fs.FS = FS(nil)
var _ fs.File = (*openMapFile)(nil)

// Open opens the named file.
func (fsys FS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	file := fsys[name]
	if file != nil && file.Mode&fs.ModeDir == 0 {
		// Ordinary file
		return &openMapFile{name, mapFileInfo{path.Base(name), file}, 0}, nil
	}

	// Directory, possibly synthesized.
	// Note that file can be nil here: the map need not contain explicit parent directories for all its files.
	// But file can also be non-nil, in case the user wants to set metadata for the directory explicitly.
	// Either way, we need to construct the list of children of this directory.
	var list []mapFileInfo
	var elem string
	var need = make(map[string]bool)
	if name == "." {
		elem = "."
		for fname, f := range fsys {
			i := strings.Index(fname, "/")
			if i < 0 {
				if fname != "." {
					list = append(list, mapFileInfo{fname, f})
				}
			} else {
				need[fname[:i]] = true
			}
		}
	} else {
		elem = name[strings.LastIndex(name, "/")+1:]
		prefix := name + "/"
		for fname, f := range fsys {
			if strings.HasPrefix(fname, prefix) {
				felem := fname[len(prefix):]
				i := strings.Index(felem, "/")
				if i < 0 {
					list = append(list, mapFileInfo{felem, f})
				} else {
					need[fname[len(prefix):len(prefix)+i]] = true
				}
			}
		}
		// If the directory name is not in the map,
		// and there are no children of the name in the map,
		// then the directory is treated as not existing.
		if file == nil && list == nil && len(need) == 0 {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
	}
	for _, fi := range list {
		delete(need, fi.name)
	}
	for name := range need {
		list = append(list, mapFileInfo{name, &MapFile{Mode: fs.ModeDir | 0555}})
	}
	slices.SortFunc(list, func(a, b mapFileInfo) int {
		return strings.Compare(a.name, b.name)
	})

	if file == nil {
		file = &MapFile{Mode: fs.ModeDir | 0555}
	}
	return &mapDir{name, mapFileInfo{elem, file}, list, 0}, nil
}

func (fsys FS) Create(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return nil, err
	}
	if ok {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrExist}
	}

	fsys[name] = &MapFile{Mode: 0666, ModTime: time.Now()}
	return &openMapFile{name, mapFileInfo{path.Base(name), fsys[name]}, 0}, nil
}

func (fsys FS) Mkdir(name string, perm fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if ok {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrExist}
	}

	ok, err = fs.Exists(fsys, path.Dir(name))
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
	}

	fsys[name] = &MapFile{Mode: perm | fs.ModeDir, ModTime: time.Now()}
	return nil
}

func (fsys FS) Chmod(name string, mode fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "chtimes", Path: name, Err: fs.ErrNotExist}
	}

	fsys[name].Mode = mode
	return nil
}

func (fsys FS) Chtimes(name string, atime, mtime time.Time) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "chtimes", Path: name, Err: fs.ErrNotExist}
	}

	fsys[name].ModTime = mtime
	return nil
}

func (fsys FS) Remove(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	// TODO: Rohit: fail if name is a directory and not empty
	// TODO: handle synthesized directories?

	delete(fsys, name)
	return nil
}

func (fsys FS) Rename(oldpath, newpath string) error {
	if !fs.ValidPath(oldpath) || !fs.ValidPath(newpath) {
		return &fs.PathError{Op: "rename", Path: oldpath, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, oldpath)
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "rename", Path: oldpath, Err: fs.ErrNotExist}
	}

	ok, err = fs.Exists(fsys, path.Dir(newpath))
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "rename", Path: newpath, Err: fs.ErrNotExist}
	}

	fsys[newpath] = fsys[oldpath]
	delete(fsys, oldpath)
	return nil
}

// A mapFileInfo implements fs.FileInfo and fs.DirEntry for a given map file.
type mapFileInfo struct {
	name string
	f    *MapFile
}

func (i *mapFileInfo) Name() string               { return path.Base(i.name) }
func (i *mapFileInfo) Size() int64                { return int64(len(i.f.Data)) }
func (i *mapFileInfo) Mode() fs.FileMode          { return i.f.Mode }
func (i *mapFileInfo) Type() fs.FileMode          { return i.f.Mode.Type() }
func (i *mapFileInfo) ModTime() time.Time         { return i.f.ModTime }
func (i *mapFileInfo) IsDir() bool                { return i.f.Mode&fs.ModeDir != 0 }
func (i *mapFileInfo) Sys() any                   { return i.f.Sys }
func (i *mapFileInfo) Info() (fs.FileInfo, error) { return i, nil }

func (i *mapFileInfo) String() string {
	return fs.FormatFileInfo(i)
}

// An openMapFile is a regular (non-directory) fs.File open for reading.
type openMapFile struct {
	path string
	mapFileInfo
	offset int64
}

func (f *openMapFile) Stat() (fs.FileInfo, error) { return &f.mapFileInfo, nil }

func (f *openMapFile) Close() error { return nil }

func (f *openMapFile) Read(b []byte) (int, error) {
	if f.offset >= int64(len(f.f.Data)) {
		return 0, io.EOF
	}
	if f.offset < 0 {
		return 0, &fs.PathError{Op: "read", Path: f.path, Err: fs.ErrInvalid}
	}
	n := copy(b, f.f.Data[f.offset:])
	f.offset += int64(n)
	return n, nil
}

func (f *openMapFile) Write(b []byte) (int, error) {
	return 0, nil
	// if f.closed == true {
	// 	return 0, fs.ErrClosed
	// }
	// if f.readOnly {
	// 	return 0, &os.PathError{Op: "write", Path: f.fileData.name, Err: errors.New("file handle is read only")}
	// }
	// n = len(b)
	// cur := atomic.LoadInt64(&f.at)
	// f.fileData.Lock()
	// defer f.fileData.Unlock()
	// diff := cur - int64(len(f.fileData.data))
	// var tail []byte
	// if n+int(cur) < len(f.fileData.data) {
	// 	tail = f.fileData.data[n+int(cur):]
	// }
	// if diff > 0 {
	// 	f.fileData.data = append(f.fileData.data, append(bytes.Repeat([]byte{00}, int(diff)), b...)...)
	// 	f.fileData.data = append(f.fileData.data, tail...)
	// } else {
	// 	f.fileData.data = append(f.fileData.data[:cur], b...)
	// 	f.fileData.data = append(f.fileData.data, tail...)
	// }
	// setModTime(f.fileData, time.Now())

	// atomic.AddInt64(&f.at, int64(n))
}

func (f *openMapFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		// offset += 0
	case 1:
		offset += f.offset
	case 2:
		offset += int64(len(f.f.Data))
	}
	if offset < 0 || offset > int64(len(f.f.Data)) {
		return 0, &fs.PathError{Op: "seek", Path: f.path, Err: fs.ErrInvalid}
	}
	f.offset = offset
	return offset, nil
}

func (f *openMapFile) ReadAt(b []byte, offset int64) (int, error) {
	if offset < 0 || offset > int64(len(f.f.Data)) {
		return 0, &fs.PathError{Op: "read", Path: f.path, Err: fs.ErrInvalid}
	}
	n := copy(b, f.f.Data[offset:])
	if n < len(b) {
		return n, io.EOF
	}
	return n, nil
}

// A mapDir is a directory fs.File (so also an fs.ReadDirFile) open for reading.
type mapDir struct {
	path string
	mapFileInfo
	entry  []mapFileInfo
	offset int
}

func (d *mapDir) Stat() (fs.FileInfo, error) { return &d.mapFileInfo, nil }
func (d *mapDir) Close() error               { return nil }
func (d *mapDir) Read(b []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.path, Err: fs.ErrInvalid}
}

func (d *mapDir) ReadDir(count int) ([]fs.DirEntry, error) {
	n := len(d.entry) - d.offset
	if n == 0 && count > 0 {
		return nil, io.EOF
	}
	if count > 0 && n > count {
		n = count
	}
	list := make([]fs.DirEntry, n)
	for i := range list {
		list[i] = &d.entry[d.offset+i]
	}
	d.offset += n
	return list, nil
}
