package fs

import (
	"context"
	"path"
)

// Sub returns an [FS] corresponding to the subtree rooted at fsys's dir.
//
// If dir is ".", Sub returns fsys unchanged.
// Otherwise, if fs implements [SubFS], Sub returns fsys.Sub(dir).
// Otherwise, Sub returns a new [FS] implementation sub that,
// in effect, implements sub.Open(name) as fsys.Open(path.Join(dir, name)).
// The implementation also translates calls to ReadDir, ReadFile, and Glob appropriately.
//
// Note that Sub(os.DirFS("/"), "prefix") is equivalent to os.DirFS("/prefix")
// and that neither of them guarantees to avoid operating system
// accesses outside "/prefix", because the implementation of [os.DirFS]
// does not check for symbolic links inside "/prefix" that point to
// other directories. That is, [os.DirFS] is not a general substitute for a
// chroot-style security mechanism, and Sub does not change that fact.
func Sub(fsys FS, dir string) (FS, error) {
	if !ValidPath(dir) {
		return nil, &PathError{Op: "sub", Path: dir, Err: ErrInvalid}
	}
	if dir == "." {
		return fsys, nil
	}
	if fsys, ok := fsys.(SubFS); ok {
		return fsys.Sub(dir)
	}
	return &SubdirFS{fsys, dir}, nil
}

type SubdirFS struct {
	Fsys FS
	Dir  string
}

// fullName maps name to the fully-qualified name dir/name.
func (f *SubdirFS) fullName(op string, name string) (string, error) {
	if !ValidPath(name) {
		return "", &PathError{Op: op, Path: name, Err: ErrInvalid}
	}
	return path.Join(f.Dir, name), nil
}

// shorten maps name, which should start with f.dir, back to the suffix after f.dir.
func (f *SubdirFS) shorten(name string) (rel string, ok bool) {
	if name == f.Dir {
		return ".", true
	}
	if len(name) >= len(f.Dir)+2 && name[len(f.Dir)] == '/' && name[:len(f.Dir)] == f.Dir {
		return name[len(f.Dir)+1:], true
	}
	return "", false
}

// fixErr shortens any reported names in PathErrors by stripping f.dir.
func (f *SubdirFS) fixErr(err error) error {
	if e, ok := err.(*PathError); ok {
		if short, ok := f.shorten(e.Path); ok {
			e.Path = short
		}
	}
	return err
}

func (f *SubdirFS) Open(name string) (File, error) {
	return f.OpenContext(context.Background(), name)
}

func (f *SubdirFS) OpenContext(ctx context.Context, name string) (File, error) {
	full, err := f.fullName("open", name)
	if err != nil {
		return nil, err
	}
	file, err := OpenContext(ctx, f.Fsys, full)
	return file, f.fixErr(err)
}

func (f *SubdirFS) Create(name string) (File, error) {
	full, err := f.fullName("create", name)
	if err != nil {
		return nil, err
	}
	return Create(f.Fsys, full)
}

func (f *SubdirFS) Mkdir(name string, perm FileMode) error {
	full, err := f.fullName("mkdir", name)
	if err != nil {
		return err
	}
	return Mkdir(f.Fsys, full, perm)
}

func (f *SubdirFS) Remove(name string) error {
	full, err := f.fullName("remove", name)
	if err != nil {
		return err
	}
	return Remove(f.Fsys, full)
}

func (f *SubdirFS) Sub(dir string) (FS, error) {
	if dir == "." {
		return f, nil
	}
	full, err := f.fullName("sub", dir)
	if err != nil {
		return nil, err
	}
	return &SubdirFS{f.Fsys, full}, nil
}
