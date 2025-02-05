package fskit

import (
	"context"
	"path"
	"slices"
	"strings"
	"time"

	"tractor.dev/wanix/fs"
)

type MemFS map[string]*Node

func (fsys MemFS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

func (fsys MemFS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	n := fsys[name]
	if n != nil {
		n.name = name
		if !n.IsDir() {
			// Ordinary file
			return fs.OpenContext(ctx, n, ".")
		}
	}

	// Directory, possibly synthesized.
	// Note that file can be nil here: the map need not contain explicit parent directories for all its files.
	// But file can also be non-nil, in case the user wants to set metadata for the directory explicitly.
	// Either way, we need to construct the list of children of this directory.
	var list []*Node
	var need = make(map[string]bool)
	if name == "." {
		for fname, fi := range fsys {
			i := strings.Index(fname, "/")
			if i < 0 {
				if fname != "." {
					list = append(list, RawNode(fi, fname))
				}
			} else {
				need[fname[:i]] = true
			}
		}
	} else {
		prefix := name + "/"
		for fname, fi := range fsys {
			if strings.HasPrefix(fname, prefix) {
				felem := fname[len(prefix):]
				i := strings.Index(felem, "/")
				if i < 0 {
					list = append(list, RawNode(fi, felem))
				} else {
					need[fname[len(prefix):len(prefix)+i]] = true
				}
			}
		}
		// If the directory name is not in the map,
		// and there are no children of the name in the map,
		// then the directory is treated as not existing.
		if n == nil && list == nil && len(need) == 0 {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
	}
	for _, fi := range list {
		delete(need, fi.name)
	}
	for name := range need {
		list = append(list, RawNode(name, fs.FileMode(fs.ModeDir|0755)))
	}
	slices.SortFunc(list, func(a, b *Node) int {
		return strings.Compare(a.Name(), b.Name())
	})

	if n == nil {
		n = RawNode(name, fs.ModeDir|0755)
	}
	var entries []fs.DirEntry
	for _, n := range list {
		entries = append(entries, n)
	}
	return DirFile(n, entries...), nil
}

func (fsys MemFS) Create(name string) (fs.File, error) {
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

	fsys[name] = Entry(name, fs.FileMode(0666), time.Now())
	return fsys[name].Open(".")
}

func (fsys MemFS) Mkdir(name string, perm fs.FileMode) error {
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

	fsys[name] = Entry(name, perm|fs.ModeDir, time.Now())
	return nil
}

func (fsys MemFS) Chmod(name string, mode fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chmod", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "chmod", Path: name, Err: fs.ErrNotExist}
	}

	fsys[name].mode = mode
	return nil
}

func (fsys MemFS) Chtimes(name string, atime, mtime time.Time) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chtimes", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "chtimes", Path: name, Err: fs.ErrNotExist}
	}

	fsys[name].modTime = mtime
	return nil
}

func (fsys MemFS) Remove(name string) error {
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

	if isDir, err := fs.IsDir(fsys, name); err != nil {
		return err
	} else if isDir {
		empty, err := fs.IsEmpty(fsys, name)
		if err != nil {
			return err
		}
		if !empty {
			return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotEmpty}
		}
	}

	// TODO: RemoveAll, gets into synthesized directories

	delete(fsys, name)
	return nil
}

func (fsys MemFS) Rename(oldpath, newpath string) error {
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
