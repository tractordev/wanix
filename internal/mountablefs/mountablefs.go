package mountablefs

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"tractor.dev/toolkit-go/engine/fs"
)

type mountedFSDir struct {
	fsys       fs.FS
	mountPoint string
}

type FS struct {
	fs.MutableFS
	mounts []mountedFSDir
}

func New(fsys fs.MutableFS) *FS {
	return &FS{MutableFS: fsys, mounts: make([]mountedFSDir, 0, 1)}
}

func (host *FS) Mount(fsys fs.FS, dirPath string) error {
	dirPath = cleanPath(dirPath)

	fi, err := fs.Stat(host, dirPath)
	if err != nil {
		return err
	}

	if !fi.IsDir() {
		return &fs.PathError{Op: "mount", Path: dirPath, Err: fs.ErrInvalid}
	}
	if found, _ := host.isPathInMount(dirPath); found {
		return &fs.PathError{Op: "mount", Path: dirPath, Err: fs.ErrExist}
	}

	host.mounts = append(host.mounts, mountedFSDir{fsys: fsys, mountPoint: dirPath})
	return nil
}

func (host *FS) Unmount(path string) error {
	path = cleanPath(path)
	for i, m := range host.mounts {
		if path == m.mountPoint {
			host.mounts = remove(host.mounts, i)
			return nil
		}
	}

	return &fs.PathError{Op: "unmount", Path: path, Err: fs.ErrInvalid}
}

func remove(s []mountedFSDir, i int) []mountedFSDir {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func (host *FS) isPathInMount(path string) (bool, *mountedFSDir) {
	for i, m := range host.mounts {
		if strings.HasPrefix(path, m.mountPoint) {
			return true, &host.mounts[i]
		}
	}
	return false, nil
}

func cleanPath(p string) string {
	return filepath.Clean(strings.TrimLeft(p, "/\\"))
}

func trimMountPoint(path string, mntPoint string) string {
	result := strings.TrimPrefix(path, mntPoint)
	result = strings.TrimPrefix(result, string(filepath.Separator))

	if result == "" {
		return "."
	} else {
		return result
	}
}

func (host *FS) Chmod(name string, mode fs.FileMode) error {
	name = cleanPath(name)
	var fsys fs.FS
	prefix := ""

	if found, mount := host.isPathInMount(name); found {
		fsys = mount.fsys
		prefix = mount.mountPoint
	} else {
		fsys = host.MutableFS
	}

	chmodableFS, ok := fsys.(interface {
		Chmod(name string, mode fs.FileMode) error
	})
	if !ok {
		return &fs.PathError{Op: "chmod", Path: name, Err: errors.ErrUnsupported}
	}
	return chmodableFS.Chmod(trimMountPoint(name, prefix), mode)
}

func (host *FS) Chown(name string, uid, gid int) error {
	name = cleanPath(name)
	var fsys fs.FS
	prefix := ""

	if found, mount := host.isPathInMount(name); found {
		fsys = mount.fsys
		prefix = mount.mountPoint
	} else {
		fsys = host.MutableFS
	}

	chownableFS, ok := fsys.(interface {
		Chown(name string, uid, gid int) error
	})
	if !ok {
		return &fs.PathError{Op: "chown", Path: name, Err: errors.ErrUnsupported}
	}
	return chownableFS.Chown(trimMountPoint(name, prefix), uid, gid)
}

func (host *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	name = cleanPath(name)
	var fsys fs.FS
	prefix := ""

	if found, mount := host.isPathInMount(name); found {
		fsys = mount.fsys
		prefix = mount.mountPoint
	} else {
		fsys = host.MutableFS
	}

	chtimesableFS, ok := fsys.(interface {
		Chtimes(name string, atime time.Time, mtime time.Time) error
	})
	if !ok {
		return &fs.PathError{Op: "chtimes", Path: name, Err: errors.ErrUnsupported}
	}
	return chtimesableFS.Chtimes(trimMountPoint(name, prefix), atime, mtime)
}

func (host *FS) Stat(name string) (fs.FileInfo, error) {
	name = cleanPath(name)
	var fsys fs.FS
	prefix := ""

	if found, mount := host.isPathInMount(name); found {
		fsys = mount.fsys
		prefix = mount.mountPoint
	} else {
		fsys = host.MutableFS
	}

	statFS, ok := fsys.(fs.StatFS)
	if !ok {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: errors.ErrUnsupported}
	}
	return statFS.Stat(trimMountPoint(name, prefix))
}

func (host *FS) Create(name string) (fs.File, error) {
	name = cleanPath(name)
	var fsys fs.FS
	prefix := ""

	if found, mount := host.isPathInMount(name); found {
		fsys = mount.fsys
		prefix = mount.mountPoint
	} else {
		fsys = host.MutableFS
	}

	createableFS, ok := fsys.(interface {
		Create(name string) (fs.File, error)
	})
	if !ok {
		return nil, &fs.PathError{Op: "create", Path: name, Err: errors.ErrUnsupported}
	}
	return createableFS.Create(trimMountPoint(name, prefix))
}

func (host *FS) Mkdir(name string, perm fs.FileMode) error {
	name = cleanPath(name)
	var fsys fs.FS
	prefix := ""

	if found, mount := host.isPathInMount(name); found {
		fsys = mount.fsys
		prefix = mount.mountPoint
	} else {
		fsys = host.MutableFS
	}

	mkdirableFS, ok := fsys.(interface {
		Mkdir(name string, perm fs.FileMode) error
	})
	if !ok {
		return &fs.PathError{Op: "mkdir", Path: name, Err: errors.ErrUnsupported}
	}
	return mkdirableFS.Mkdir(trimMountPoint(name, prefix), perm)
}

func (host *FS) MkdirAll(path string, perm fs.FileMode) error {
	path = cleanPath(path)
	var fsys fs.FS
	prefix := ""

	if found, mount := host.isPathInMount(path); found {
		fsys = mount.fsys
		prefix = mount.mountPoint
	} else {
		fsys = host.MutableFS
	}

	mkdirableFS, ok := fsys.(interface {
		MkdirAll(path string, perm fs.FileMode) error
	})
	if !ok {
		return &fs.PathError{Op: "mkdirAll", Path: path, Err: errors.ErrUnsupported}
	}
	return mkdirableFS.MkdirAll(trimMountPoint(path, prefix), perm)
}

func (host *FS) Open(name string) (fs.File, error) {
	name = cleanPath(name)
	if found, mount := host.isPathInMount(name); found {
		return mount.fsys.Open(trimMountPoint(name, mount.mountPoint))
	}

	return host.MutableFS.Open(name)
}

func (host *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	if found, mount := host.isPathInMount(name); found {
		return fs.OpenFile(mount.fsys, trimMountPoint(name, mount.mountPoint), flag, perm)
	} else {
		return fs.OpenFile(host.MutableFS, name, flag, perm)
	}
}

type removableFS interface {
	fs.FS
	Remove(name string) error
}

func (host *FS) Remove(name string) error {
	name = cleanPath(name)
	var fsys fs.FS
	prefix := ""

	if found, mount := host.isPathInMount(name); found {
		if name == mount.mountPoint {
			return &fs.PathError{Op: "remove", Path: name, Err: syscall.EBUSY}
		}

		fsys = mount.fsys
		prefix = mount.mountPoint
	} else {
		fsys = host.MutableFS
	}

	if removableFS, ok := fsys.(removableFS); ok {
		return removableFS.Remove(trimMountPoint(name, prefix))
	} else {
		return &fs.PathError{Op: "remove", Path: name, Err: errors.ErrUnsupported}
	}
}

func (host *FS) RemoveAll(path string) error {
	path = cleanPath(path)
	var fsys fs.FS
	prefix := ""

	if found, mount := host.isPathInMount(path); found {
		if path == mount.mountPoint {
			return &fs.PathError{Op: "removeAll", Path: path, Err: syscall.EBUSY}
		}

		fsys = mount.fsys
		prefix = mount.mountPoint
	} else {
		fsys = host.MutableFS
		// check if path contains any mountpoints, and call a custom removeAll
		// if it does.
		var mntPoints []string
		for _, m := range host.mounts {
			if path == "." || strings.HasPrefix(m.mountPoint, path) {
				mntPoints = append(mntPoints, m.mountPoint)
			}
		}

		if len(mntPoints) > 0 {
			return removeAll(host, path, mntPoints)
		}
	}

	rmAllFS, ok := fsys.(interface {
		RemoveAll(path string) error
	})
	if !ok {
		if rmFS, ok := fsys.(removableFS); ok {
			return removeAll(rmFS, path, nil)
		} else {
			return &fs.PathError{Op: "removeAll", Path: path, Err: errors.ErrUnsupported}
		}
	}
	return rmAllFS.RemoveAll(trimMountPoint(path, prefix))
}

// RemoveAll removes path and any children it contains. It removes everything
// it can but returns the first error it encounters. If the path does not exist,
// RemoveAll returns nil (no error). If there is an error, it will be of type *PathError.
// Additionally, this function errors if attempting to remove a mountpoint.
func removeAll(fsys removableFS, path string, mntPoints []string) error {
	path = filepath.Clean(path)

	if exists, err := fs.Exists(fsys, path); !exists || err != nil {
		return err
	}

	return rmRecurse(fsys, path, mntPoints)

}

func rmRecurse(fsys removableFS, path string, mntPoints []string) error {
	if mntPoints != nil && slices.Contains(mntPoints, path) {
		return &fs.PathError{Op: "remove", Path: path, Err: syscall.EBUSY}
	}

	isdir, dirErr := fs.IsDir(fsys, path)
	if dirErr != nil {
		return dirErr
	}

	if isdir {
		if entries, err := fs.ReadDir(fsys, path); err == nil {
			for _, entry := range entries {
				entryPath := filepath.Join(path, entry.Name())

				if err := rmRecurse(fsys, entryPath, mntPoints); err != nil {
					return err
				}

				if err := fsys.Remove(entryPath); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}

	return fsys.Remove(path)
}

func (host *FS) Rename(oldname, newname string) error {
	oldname = cleanPath(oldname)
	newname = cleanPath(newname)
	var fsys fs.FS
	prefix := ""

	// error if both paths aren't in the same filesystem
	if found, oldMount := host.isPathInMount(oldname); found {
		if found, newMount := host.isPathInMount(newname); found {
			if oldMount != newMount {
				return &fs.PathError{Op: "rename", Path: oldname + " -> " + newname, Err: syscall.EXDEV}
			}

			if oldname == oldMount.mountPoint || newname == newMount.mountPoint {
				return &fs.PathError{Op: "rename", Path: oldname + " -> " + newname, Err: syscall.EBUSY}
			}

			fsys = newMount.fsys
			prefix = newMount.mountPoint
		} else {
			return &fs.PathError{Op: "rename", Path: oldname + " -> " + newname, Err: syscall.EXDEV}
		}
	} else {
		if found, _ := host.isPathInMount(newname); found {
			return &fs.PathError{Op: "rename", Path: oldname + " -> " + newname, Err: syscall.EXDEV}
		}

		fsys = host.MutableFS
	}

	renameableFS, ok := fsys.(interface {
		Rename(oldname, newname string) error
	})
	if !ok {
		return &fs.PathError{Op: "rename", Path: oldname + " -> " + newname, Err: errors.ErrUnsupported}
	}
	return renameableFS.Rename(trimMountPoint(oldname, prefix), trimMountPoint(newname, prefix))
}
