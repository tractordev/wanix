package fs

import (
	"context"
	"path"
	"time"
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

func (f *SubdirFS) Stat(name string) (FileInfo, error) {
	return f.StatContext(context.Background(), name)
}

func (f *SubdirFS) StatContext(ctx context.Context, name string) (FileInfo, error) {
	full, err := f.fullName("stat", name)
	if err != nil {
		return nil, err
	}
	info, err := StatContext(ctx, f.Fsys, full)
	return info, f.fixErr(err)
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
	file, err := Create(f.Fsys, full)
	return file, f.fixErr(err)
}

func (f *SubdirFS) Mkdir(name string, perm FileMode) error {
	full, err := f.fullName("mkdir", name)
	if err != nil {
		return err
	}
	return f.fixErr(Mkdir(f.Fsys, full, perm))
}

func (f *SubdirFS) Truncate(name string, size int64) error {
	full, err := f.fullName("truncate", name)
	if err != nil {
		return err
	}
	return f.fixErr(Truncate(f.Fsys, full, size))
}

func (f *SubdirFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	full, err := f.fullName("chtimes", name)
	if err != nil {
		return err
	}
	return f.fixErr(Chtimes(f.Fsys, full, atime, mtime))
}

func (f *SubdirFS) Chmod(name string, mode FileMode) error {
	full, err := f.fullName("chmod", name)
	if err != nil {
		return err
	}
	return f.fixErr(Chmod(f.Fsys, full, mode))
}

func (f *SubdirFS) Remove(name string) error {
	full, err := f.fullName("remove", name)
	if err != nil {
		return err
	}
	return f.fixErr(Remove(f.Fsys, full))
}

func (f *SubdirFS) Rename(oldname string, newname string) error {
	newfull, err := f.fullName("rename", newname)
	if err != nil {
		return err
	}
	oldfull, err := f.fullName("rename", oldname)
	if err != nil {
		return err
	}
	return f.fixErr(Rename(f.Fsys, oldfull, newfull))
}

func (f *SubdirFS) Symlink(oldname string, newname string) error {
	full, err := f.fullName("symlink", newname)
	if err != nil {
		return err
	}
	return f.fixErr(Symlink(f.Fsys, oldname, full))
}

func (f *SubdirFS) Readlink(name string) (string, error) {
	full, err := f.fullName("readlink", name)
	if err != nil {
		return "", err
	}
	link, err := Readlink(f.Fsys, full)
	return link, f.fixErr(err)
}

func (f *SubdirFS) SetXattr(ctx context.Context, name string, attr string, data []byte, flags int) error {
	full, err := f.fullName("setxattr", name)
	if err != nil {
		return err
	}
	return f.fixErr(SetXattr(ctx, f.Fsys, full, attr, data, flags))
}

func (f *SubdirFS) GetXattr(ctx context.Context, name string, attr string) ([]byte, error) {
	full, err := f.fullName("getxattr", name)
	if err != nil {
		return nil, err
	}
	return GetXattr(ctx, f.Fsys, full, attr)
}

func (f *SubdirFS) ListXattrs(ctx context.Context, name string) ([]string, error) {
	full, err := f.fullName("listxattrs", name)
	if err != nil {
		return nil, err
	}
	return ListXattrs(ctx, f.Fsys, full)
}

func (f *SubdirFS) RemoveXattr(ctx context.Context, name string, attr string) error {
	full, err := f.fullName("removexattr", name)
	if err != nil {
		return err
	}
	return f.fixErr(RemoveXattr(ctx, f.Fsys, full, attr))
}

// TODO: watch
// TODO: chown
