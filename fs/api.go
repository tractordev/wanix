package fs

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"time"
)

type OpenFileFS interface {
	FS
	OpenFile(name string, flag int, perm FileMode) (File, error)
}

// OpenFile is a helper that opens a file with the given flag and permissions if supported.
func OpenFile(fsys FS, name string, flag int, perm FileMode) (File, error) {
	if o, ok := fsys.(OpenFileFS); ok {
		return o.OpenFile(name, flag, perm)
	}

	// Log all open flags
	// log.Printf("openfile flags: O_RDONLY=%v O_WRONLY=%v O_RDWR=%v O_APPEND=%v O_CREATE=%v O_EXCL=%v O_SYNC=%v O_TRUNC=%v",
	// 	flag&os.O_RDONLY != 0,
	// 	flag&os.O_WRONLY != 0,
	// 	flag&os.O_RDWR != 0,
	// 	flag&os.O_APPEND != 0,
	// 	flag&os.O_CREATE != 0,
	// 	flag&os.O_EXCL != 0,
	// 	flag&os.O_SYNC != 0,
	// 	flag&os.O_TRUNC != 0)

	// if create flag is set
	if flag&os.O_CREATE != 0 {
		if flag&os.O_APPEND == 0 {
			// if not append, create a new file
			return Create(fsys, name)
		} else {
			// if append, open the file
			f, err := fsys.Open(name)
			if err != nil {
				// if file doesn't exist, create it
				if os.IsNotExist(err) {
					return Create(fsys, name)
				}
				return nil, err
			}
			// todo: seek to the end?
			return f, nil
		}
	}

	// just fall back to Open
	return fsys.Open(name)
}

type ChmodFS interface {
	FS
	Chmod(name string, mode FileMode) error
}

// Chmod changes the mode of the named file if supported.
func Chmod(fsys FS, name string, mode FileMode) error {
	if c, ok := fsys.(ChmodFS); ok {
		return c.Chmod(name, mode)
	}
	return ErrNotSupported
}

type ChownFS interface {
	FS
	Chown(name string, uid, gid int) error
}

// Chown changes the numeric uid and gid of the named file if supported.
func Chown(fsys FS, name string, uid, gid int) error {
	if c, ok := fsys.(ChownFS); ok {
		return c.Chown(name, uid, gid)
	}
	return ErrNotSupported
}

type ChtimesFS interface {
	FS
	Chtimes(name string, atime time.Time, mtime time.Time) error
}

// Chtimes changes the access and modification times of the named file if supported.
func Chtimes(fsys FS, name string, atime time.Time, mtime time.Time) error {
	if c, ok := fsys.(ChtimesFS); ok {
		return c.Chtimes(name, atime, mtime)
	}

	if path.Dir(name) != "." {
		parent, err := Sub(fsys, path.Dir(name))
		if err != nil {
			return err
		}
		if subfs, ok := parent.(*SubdirFS); ok && reflect.DeepEqual(subfs.Fsys, fsys) {
			// if parent is a SubdirFS of our fsys, we manually
			// call Chtimes to avoid infinite recursion
			full, err := subfs.fullName("chtimes", path.Base(name))
			if err != nil {
				return err
			}
			if m, ok := subfs.Fsys.(ChtimesFS); ok {
				return m.Chtimes(full, atime, mtime)
			}
			return fmt.Errorf("%w on %T: Chtimes %s", ErrNotSupported, subfs.Fsys, full)
		}
		return Chtimes(parent, path.Base(name), atime, mtime)
	}

	return ErrNotSupported
}

type RenameFS interface {
	FS
	Rename(oldname, newname string) error
}

// Rename renames (moves) oldname to newname if supported.
func Rename(fsys FS, oldname, newname string) error {
	if r, ok := fsys.(RenameFS); ok {
		return r.Rename(oldname, newname)
	}
	return ErrNotSupported
}
