package localfs

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/pstat"
)

type FS struct {
	baseDir          string
	virtualizeUidGid bool
	chownData        map[string][2]int // path -> [uid, gid]
	chownMutex       sync.RWMutex
	log              *slog.Logger

	create      func(name string) (fs.File, error)
	mkdir       func(name string, perm fs.FileMode) error
	mkdirAll    func(path string, perm fs.FileMode) error
	openContext func(ctx context.Context, name string) (fs.File, error)
	openFile    func(name string, flag int, perm fs.FileMode) (fs.File, error)
	remove      func(name string) error
	removeAll   func(path string) error
	rename      func(oldname, newname string) error
	stat        func(name string) (fs.FileInfo, error)
	lstat       func(name string) (fs.FileInfo, error)
	chmod       func(name string, mode fs.FileMode) error
	chown       func(name string, uid, gid int) error
	chtimes     func(name string, atime time.Time, mtime time.Time) error
	symlink     func(oldname, newname string) error
	readlink    func(name string) (string, error)
}

func New(dir string) (*FS, error) {
	return newRoot(dir, &FS{
		virtualizeUidGid: false,
		chownData:        make(map[string][2]int),
		log:              slog.Default(), // for now
	})
}

// SetLogger sets the logger for the filesystem
func (fsys *FS) SetLogger(logger *slog.Logger) {
	fsys.log = logger
}

// virtualFileInfo wraps a FileInfo to virtualize uid/gid values
type virtualFileInfo struct {
	fs.FileInfo
	fsys *FS
	path string
}

func (vfi *virtualFileInfo) Sys() interface{} {
	if !vfi.fsys.virtualizeUidGid {
		return vfi.FileInfo.Sys()
	}

	// Check if we have custom chown data for this path
	vfi.fsys.chownMutex.RLock()
	customOwnership, hasCustom := vfi.fsys.chownData[vfi.path]
	vfi.fsys.chownMutex.RUnlock()

	// Use the stat package to get a portable stat structure
	origStat := pstat.FileInfoToStat(vfi.FileInfo)
	if origStat == nil {
		panic("pstat.FileInfoToStat returned nil")
	}

	// Create a copy of the stat and override uid/gid
	newStat := *origStat
	if hasCustom {
		newStat.Uid = uint32(customOwnership[0])
		newStat.Gid = uint32(customOwnership[1])
	} else {
		// Default to 0:0 when virtualizing
		newStat.Uid = 0
		newStat.Gid = 0
	}
	return pstat.StatToSys(&newStat)
}

func (fsys *FS) wrapFileInfo(fi fs.FileInfo, path string) fs.FileInfo {
	if !fsys.virtualizeUidGid {
		return fi
	}
	return &virtualFileInfo{
		FileInfo: fi,
		fsys:     fsys,
		path:     path,
	}
}

func (fsys *FS) Create(name string) (fs.File, error) {
	fsys.log.Debug("Create", "name", name)
	f, e := fsys.create(name)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (fsys *FS) Mkdir(name string, perm fs.FileMode) error {
	fsys.log.Debug("Mkdir", "name", name, "perm", perm)
	return fsys.mkdir(name, perm)
}

func (fsys *FS) MkdirAll(path string, perm fs.FileMode) error {
	fsys.log.Debug("MkdirAll", "path", path, "perm", perm)
	return fsys.mkdirAll(path, perm)
}

func (fsys *FS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

func (fsys *FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys.log.Debug("Open", "name", name)
	f, e := fsys.openContext(ctx, name)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (fsys *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	fsys.log.Debug("OpenFile", "name", name, "flag", flag, "perm", perm)
	f, e := fsys.openFile(name, flag, perm)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (fsys *FS) Remove(name string) error {
	fsys.log.Debug("Remove", "name", name)
	return fsys.remove(name)
}

func (fsys *FS) RemoveAll(path string) error {
	fsys.log.Debug("RemoveAll", "path", path)
	return fsys.removeAll(path)
}

func (fsys *FS) Rename(oldname, newname string) error {
	fsys.log.Debug("Rename", "oldname", oldname, "newname", newname)
	return fsys.rename(oldname, newname)
}

func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	return fsys.StatContext(context.Background(), name)
}

func (fsys *FS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	fsys.log.Debug("Stat", "name", name)
	var fi fs.FileInfo
	var err error

	if fs.FollowSymlinks(ctx) {
		fi, err = fsys.stat(name)
	} else {
		fi, err = fsys.lstat(name)
	}

	if err != nil {
		return nil, err
	}

	return fsys.wrapFileInfo(fi, name), nil
}

func (fsys *FS) Chmod(name string, mode fs.FileMode) error {
	fsys.log.Debug("Chmod", "name", name, "mode", mode)
	return fsys.chmod(name, mode)
}

func (fsys *FS) Chown(name string, uid, gid int) error {
	fsys.log.Debug("Chown", "name", name, "uid", uid, "gid", gid)
	if fsys.virtualizeUidGid {
		// Store chown data in memory instead of applying to filesystem
		fsys.chownMutex.Lock()
		fsys.chownData[name] = [2]int{uid, gid}
		fsys.chownMutex.Unlock()
		return nil
	}
	return fsys.chown(name, uid, gid)
}

func (fsys *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	fsys.log.Debug("Chtimes", "name", name, "atime", atime, "mtime", mtime)
	return fsys.chtimes(name, atime, mtime)
}

func (fsys *FS) Symlink(oldname string, newname string) error {
	fsys.log.Debug("Symlink", "oldname", oldname, "newname", newname)
	return fsys.symlink(oldname, newname)
}

func (fsys *FS) Readlink(name string) (string, error) {
	fsys.log.Debug("Readlink", "name", name)
	return fsys.readlink(name)
}
