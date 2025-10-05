//go:build !wasm

package localfs

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/pstat"
)

type FS struct {
	root             *os.Root
	virtualizeUidGid bool
	chownData        map[string][2]int // path -> [uid, gid]
	chownMutex       sync.RWMutex
	log              *slog.Logger
}

func New(dir string) (*FS, error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &FS{
		root:             r,
		virtualizeUidGid: false,
		chownData:        make(map[string][2]int),
		log:              slog.Default(), // for now
	}, nil
}

// NewWithVirtualUidGid creates a new localfs that virtualizes all uid/gid to 0:0
// and stores chown operations in memory instead of applying them to the filesystem
func NewWithVirtualUidGid(dir string) (*FS, error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &FS{
		root:             r,
		virtualizeUidGid: true,
		chownData:        make(map[string][2]int),
		log:              slog.Default(), // for now
	}, nil
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
	f, e := fsys.root.Create(name)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (fsys *FS) Mkdir(name string, perm fs.FileMode) error {
	return fsys.root.Mkdir(name, perm)
}

func (fsys *FS) MkdirAll(path string, perm fs.FileMode) error {
	return fsys.root.MkdirAll(path, perm)
}

func (fsys *FS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

func (fsys *FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	f, e := fsys.root.Open(name)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (fsys *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	f, e := fsys.root.OpenFile(name, flag, perm)
	if f == nil {
		// while this looks strange, we need to return a bare nil (of type nil) not
		// a nil value of type *os.File or nil won't be nil
		return nil, e
	}
	return f, e
}

func (fsys *FS) Remove(name string) error {
	return fsys.root.Remove(name)
}

func (fsys *FS) RemoveAll(path string) error {
	return fsys.root.RemoveAll(path)
}

func (fsys *FS) Rename(oldname, newname string) error {
	return fsys.root.Rename(oldname, newname)
}

func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	return fsys.StatContext(context.Background(), name)
}

func (fsys *FS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	var fi fs.FileInfo
	var err error

	if fs.FollowSymlinks(ctx) {
		fi, err = fsys.root.Stat(name)
	} else {
		fi, err = fsys.root.Lstat(name)
	}

	if err != nil {
		return nil, err
	}

	return fsys.wrapFileInfo(fi, name), nil
}

func (fsys *FS) Chmod(name string, mode fs.FileMode) error {
	return fsys.root.Chmod(name, mode)
}

func (fsys *FS) Chown(name string, uid, gid int) error {
	if fsys.virtualizeUidGid {
		// Store chown data in memory instead of applying to filesystem
		fsys.chownMutex.Lock()
		fsys.chownData[name] = [2]int{uid, gid}
		fsys.chownMutex.Unlock()
		return nil
	}
	return fsys.root.Chown(name, uid, gid)
}

func (fsys *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return fsys.root.Chtimes(name, atime, mtime)
}

func (fsys *FS) Symlink(oldname string, newname string) error {
	return fsys.root.Symlink(oldname, newname)
}

func (fsys *FS) Readlink(name string) (string, error) {
	return fsys.root.Readlink(name)
}
