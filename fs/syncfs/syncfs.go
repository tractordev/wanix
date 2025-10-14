package syncfs

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

type IndexFS interface {
	fs.FS
	Index(ctx context.Context, name string) (fs.FS, error)
}

type PatchFS interface {
	fs.FS
	Patch(ctx context.Context, name string, tarBuf bytes.Buffer) error
}

type RemoteFS interface {
	IndexFS
	PatchFS
}

// call recursive watch on remote
// triggers initial sync
// - lock writes
// - synchronous dir structure (depth first)
// - unlock writes
// - async file downloads (breadth first)
// - per file waitgroup
// batcher: on batch patch
// - invalidate+sync
// - fire watch event
// - reset polling timer

// SyncFS provides a filesystem that syncs between local and remote filesystems.
// All read operations go to the local filesystem first, with smart caching of remote entries.
// Write operations are applied locally immediately and queued for remote processing.
type SyncFS struct {
	local  fs.FS    // Local filesystem (fast access)
	remote RemoteFS // Remote filesystem (slower, authoritative)

	writeLock *sync.WaitGroup
	changes   map[string]bool
	debounce  *time.Timer
	mu        sync.Mutex

	log *slog.Logger
}

// New creates a new SyncFS with the given local and remote filesystems
func New(local fs.FS, remote RemoteFS) *SyncFS {
	sfs := &SyncFS{
		local:  local,
		remote: remote,
		log:    slog.Default(), // for now
	}
	// go sfs.syncWorker()
	return sfs
}

func (sfs *SyncFS) Sync() error {
	sfs.log.Debug("Sync:start")
	startTime := time.Now()
	sfs.writeLock = &sync.WaitGroup{}
	sfs.writeLock.Add(1)
	defer func() {
		sfs.writeLock.Done()
		sfs.writeLock = nil
		sfs.log.Debug("Sync:finish", "dur", time.Since(startTime))
	}()

	rindex, err := sfs.remote.Index(context.Background(), ".")
	if err != nil {
		return err
	}

	var scanStep sync.WaitGroup
	var pullDirs []string
	var pullFiles []string

	pullScan := make(chan error, 1)
	scanStep.Add(1)
	go func() {
		defer scanStep.Done()
		pullScan <- fs.WalkDir(rindex, ".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == "." {
				return nil
			}
			sfs.mu.Lock()
			exists, ok := sfs.changes[path]
			sfs.mu.Unlock()
			if ok && !exists {
				// tombstoned
				return nil
			}
			rinfo, err := entry.Info()
			if err != nil {
				return err
			}
			linfo, err := fs.Stat(sfs.local, path)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if os.IsNotExist(err) || linfo.ModTime().Unix() < rinfo.ModTime().Unix() {
				if rinfo.ModTime().Unix()-linfo.ModTime().Unix() < 2 {
					return nil
				}
				if rinfo.IsDir() {
					pullDirs = append(pullDirs, path)
				} else {
					pullFiles = append(pullFiles, path)
				}
			}
			return nil
		})
	}()

	pushScan := make(chan error, 1)
	if sfs.changes == nil {
		sfs.changes = make(map[string]bool)
		scanStep.Add(1)
		go func() {
			defer scanStep.Done()
			pushScan <- fs.WalkDir(sfs.local, ".", func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if path == "." {
					return nil
				}
				linfo, err := entry.Info()
				if err != nil {
					return err
				}
				rinfo, err := fs.Stat(rindex, path)
				if err != nil && !errors.Is(err, fs.ErrNotExist) {
					return err
				}
				if errors.Is(err, fs.ErrNotExist) || rinfo.ModTime().Unix() < linfo.ModTime().Unix() {
					sfs.changes[path] = true
				}
				return nil
			})
		}()
	} else {
		pushScan <- nil
	}

	scanStep.Wait()
	if err := <-pullScan; err != nil {
		return err
	}
	if err := <-pushScan; err != nil {
		return err
	}

	sfs.log.Debug("Sync:remote-diff", "dirs", len(pullDirs), "files", len(pullFiles))
	sfs.log.Debug("Sync:local-diff", "changes", len(sfs.changes))

	var syncStep sync.WaitGroup

	pushSync := make(chan error, 1)
	if len(sfs.changes) > 0 {
		syncStep.Add(1)
		go func() {
			defer syncStep.Done()
			var tarBuf bytes.Buffer
			tw := tar.NewWriter(&tarBuf)
			for path, exists := range sfs.changes {
				if !exists {
					header := &tar.Header{
						Name: path,
						Mode: 0,
						Size: 0,
					}
					header.PAXRecords = make(map[string]string)
					header.PAXRecords["delete"] = ""
					if err := tw.WriteHeader(header); err != nil {
						pushSync <- err
						return
					}
					continue
				}

				info, err := fs.Stat(sfs.local, path)
				if err != nil {
					pushSync <- err
					return
				}

				header, err := tar.FileInfoHeader(info, "")
				if err != nil {
					pushSync <- err
					return
				}
				header.Name = path

				// Handle symlinks
				if info.Mode()&fs.ModeSymlink != 0 {
					link, err := fs.Readlink(sfs.local, path)
					if err != nil {
						pushSync <- err
						return
					}
					header.Linkname = link
				}

				if err := tw.WriteHeader(header); err != nil {
					pushSync <- err
					return
				}

				if !info.Mode().IsRegular() {
					continue
				}

				f, err := sfs.local.Open(path)
				if err != nil {
					pushSync <- err
					return
				}
				defer f.Close()

				_, err = io.Copy(tw, f)
				if err != nil {
					pushSync <- err
					return
				}
			}
			tw.Close()
			if err := sfs.remote.Patch(context.Background(), ".", tarBuf); err != nil {
				pushSync <- err
				return
			}
			sfs.mu.Lock()
			sfs.changes = make(map[string]bool)
			sfs.mu.Unlock()
			pushSync <- nil
		}()
	} else {
		pushSync <- nil
	}

	workers := 32
	pullSync := make(chan error, workers)
	syncStep.Add(1)
	go func() {
		defer syncStep.Done()
		for _, path := range pullDirs {
			info, err := fs.Stat(rindex, path)
			if err != nil {
				pullSync <- err
				return
			}
			if err := fs.MkdirAll(sfs.local, path, info.Mode().Perm()); err != nil {
				pullSync <- err
				return
			}
		}
		worker := func(paths chan string, wg *sync.WaitGroup) {
			defer wg.Done()
			for path := range paths {
				if err := fs.CopyFS(sfs.remote, path, sfs.local, path); err != nil {
					sfs.log.Error("CopyFS", "err", err, "path", path)
					pullSync <- err
					return
				}
				info, err := fs.Stat(rindex, path)
				if err != nil {
					pullSync <- err
					return
				}
				if err := fs.Chtimes(sfs.local, path, info.ModTime(), info.ModTime()); err != nil {
					pullSync <- err
					return
				}
			}
		}
		paths := make(chan string)
		var wg sync.WaitGroup
		for i := 1; i <= workers; i++ {
			wg.Add(1)
			go worker(paths, &wg)
		}
		go func() {
			for _, path := range pullFiles {
				paths <- path
			}
			close(paths)
		}()
		wg.Wait()
		for _, path := range pullDirs {
			info, err := fs.Stat(rindex, path)
			if err != nil {
				pullSync <- err
				return
			}
			if err := fs.Chtimes(sfs.local, path, info.ModTime(), info.ModTime()); err != nil {
				pullSync <- err
				return
			}
		}
		pullSync <- nil
	}()

	syncStep.Wait()

	if err := <-pullSync; err != nil {
		return err
	}
	if err := <-pushSync; err != nil {
		return err
	}

	return nil
}

func (sfs *SyncFS) clean(path string) string {
	cleanPath := strings.TrimPrefix(filepath.Clean(path), "/")
	if cleanPath == "" {
		cleanPath = "."
	}
	return cleanPath
}

func (sfs *SyncFS) wait() {
	sfs.mu.Lock()
	lock := sfs.writeLock
	sfs.mu.Unlock()
	if lock == nil {
		return
	}
	lock.Wait()
}

func (sfs *SyncFS) changed(name string, exists bool) {
	sfs.mu.Lock()
	defer sfs.mu.Unlock()
	didExist, ok := sfs.changes[name]
	if ok && didExist && !exists {
		delete(sfs.changes, name)
		return
	}
	sfs.changes[name] = exists
	if sfs.debounce != nil {
		sfs.debounce.Stop()
	}
	sfs.debounce = time.AfterFunc(3*time.Second, func() {
		sfs.Sync()
	})
}

// Close shuts down the SyncFS and waits for all operations to complete
func (sfs *SyncFS) Close() error {
	// sfs.writeLocks.Range(func(key, value interface{}) bool {
	// 	value.(*sync.WaitGroup).Wait()
	// 	return true
	// })
	return nil
}

// Filesystem interface implementations

// Open opens a file for reading, always trying local first
func (sfs *SyncFS) Open(name string) (fs.File, error) {
	return sfs.OpenContext(context.Background(), name)
}

// OpenContext opens a file with context, trying local first.
// if local is found and dircache is expired, trigger async pull.
// if local is not found, trigger async pull. (should be sync?)
// todo: if local not found, and dircache is expired, do a sync pull/readdir
// THEN if not in dircache, return error. if in dircache, wait for local.
func (sfs *SyncFS) OpenContext(ctx context.Context, name string) (f fs.File, err error) {
	name = sfs.clean(name)
	defer func() {
		sfs.log.Debug("Open", "name", name, "err", err)
	}()
	f, err = fs.OpenContext(ctx, sfs.local, name)
	return
}

// Stat returns file info, trying local first
func (sfs *SyncFS) Stat(name string) (fs.FileInfo, error) {
	return sfs.StatContext(context.Background(), name)
}

// StatContext returns file info with context
func (sfs *SyncFS) StatContext(ctx context.Context, name string) (info fs.FileInfo, err error) {
	name = sfs.clean(name)
	defer func() {
		sfs.log.Debug("Stat", "name", name, "err", err)
	}()
	info, err = fs.StatContext(ctx, sfs.local, name)
	return
}

// ReadDir reads directory entries, with smart caching
func (sfs *SyncFS) ReadDir(name string) (entries []fs.DirEntry, err error) {
	name = sfs.clean(name)
	defer func() {
		sfs.log.Debug("ReadDir", "name", name, "entries", len(entries), "err", err)
	}()
	entries, err = fs.ReadDir(sfs.local, name)
	return
}

// Write operations - all apply locally immediately and queue for remote

// Create creates a new file
func (sfs *SyncFS) Create(name string) (fs.File, error) {
	name = sfs.clean(name)
	sfs.wait()
	defer sfs.changed(name, true)
	sfs.log.Debug("Create", "name", name)

	f, err := fs.Create(sfs.local, name)
	if err != nil {
		return nil, err
	}

	return sfs.newSyncFile(f, name)
}

// OpenFile opens a file with flags
// TODO: move opencontext here and forward?
func (sfs *SyncFS) OpenFile(name string, flag int, perm fs.FileMode) (f fs.File, err error) {
	name = sfs.clean(name)
	sfs.wait()
	defer func() {
		sfs.log.Debug("OpenFile", "name", name, "flag", flag, "perm", perm, "err", err)
	}()

	f, err = fs.OpenFile(sfs.local, name, flag, perm)
	if err != nil {
		return nil, err
	}

	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0 {
		if flag&os.O_CREATE != 0 {
			defer sfs.changed(name, true)
		}
	}

	return sfs.newSyncFile(f, name)
}

// Mkdir creates a directory
func (sfs *SyncFS) Mkdir(name string, perm fs.FileMode) error {
	name = sfs.clean(name)
	sfs.wait()
	defer sfs.changed(name, true)
	sfs.log.Debug("Mkdir", "name", name, "perm", perm)

	err := fs.Mkdir(sfs.local, name, perm)
	if err != nil {
		return err
	}

	return nil
}

func (sfs *SyncFS) Remove(name string) error {
	name = sfs.clean(name)
	sfs.wait()
	defer sfs.changed(name, false)
	sfs.log.Debug("Remove", "name", name)

	err := fs.Remove(sfs.local, name)
	if err != nil {
		return err
	}

	return nil
}

func (sfs *SyncFS) Rename(oldname, newname string) error {
	newname = sfs.clean(newname)
	oldname = sfs.clean(oldname)
	sfs.wait()
	defer sfs.changed(newname, true)
	defer sfs.changed(oldname, false)
	sfs.log.Debug("Rename", "oldname", oldname, "newname", newname)

	err := fs.Rename(sfs.local, oldname, newname)
	if err != nil {
		return err
	}

	return nil
}

func (sfs *SyncFS) Chmod(name string, mode fs.FileMode) error {
	name = sfs.clean(name)
	sfs.wait()
	defer sfs.changed(name, true)
	sfs.log.Debug("Chmod", "name", name, "mode", mode)

	err := fs.Chmod(sfs.local, name, mode)
	if err != nil {
		return err
	}

	return nil
}

func (sfs *SyncFS) Chown(name string, uid, gid int) error {
	name = sfs.clean(name)
	sfs.wait()
	defer sfs.changed(name, true)
	sfs.log.Debug("Chown", "name", name, "uid", uid, "gid", gid)

	err := fs.Chown(sfs.local, name, uid, gid)
	if err != nil {
		return err
	}

	return nil
}

func (sfs *SyncFS) Chtimes(name string, atime, mtime time.Time) error {
	name = sfs.clean(name)
	sfs.wait()
	defer sfs.changed(name, true)
	sfs.log.Debug("Chtimes", "name", name, "atime", atime, "mtime", mtime)

	err := fs.Chtimes(sfs.local, name, atime, mtime)
	if err != nil {
		return err
	}

	return nil
}

// syncFile wraps a local fs.File to detect writes and sync to remote on close
type syncFile struct {
	fs.File
	sfs       *SyncFS
	path      string
	modified  bool
	isDir     bool
	writeTime time.Time // Track when the first write occurred

	iter *fskit.DirIter
}

// newSyncFile creates a wrapped file that syncs changes to remote
// todo: newLocalSyncFile?
func (sfs *SyncFS) newSyncFile(f fs.File, path string) (*syncFile, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	return &syncFile{
		File:     f,
		sfs:      sfs,
		path:     sfs.clean(path),
		modified: false,
		isDir:    info.IsDir(),
	}, nil
}

// Write wraps the underlying Write and marks file as modified
func (sf *syncFile) Write(p []byte) (int, error) {
	if w, ok := sf.File.(interface{ Write([]byte) (int, error) }); ok {
		n, err := w.Write(p)
		if err == nil && n > 0 {
			if !sf.modified {
				// Capture the time of the first write
				sf.writeTime = time.Now()
				sf.modified = true
			}
		}
		return n, err
	}
	return 0, fs.ErrPermission
}

// WriteAt wraps the underlying WriteAt and marks file as modified
func (sf *syncFile) WriteAt(p []byte, off int64) (int, error) {
	if wa, ok := sf.File.(interface {
		WriteAt([]byte, int64) (int, error)
	}); ok {
		n, err := wa.WriteAt(p, off)
		if err == nil && n > 0 {
			if !sf.modified {
				// Capture the time of the first write
				sf.writeTime = time.Now()
				sf.modified = true
			}
		}
		return n, err
	}
	return 0, fs.ErrPermission
}

func (sf *syncFile) Seek(offset int64, whence int) (int64, error) {
	if s, ok := sf.File.(interface {
		Seek(int64, int) (int64, error)
	}); ok {
		return s.Seek(offset, whence)
	}
	return 0, fs.ErrPermission
}

// ReadDir implements fs.ReadDirFile using SyncFS ReadDir with proper cursor management
func (sf *syncFile) ReadDir(n int) (entries []fs.DirEntry, err error) {
	defer func() {
		sf.sfs.log.Debug("ReadDir", "path", sf.path, "n", n, "entries", len(entries), "err", err)
	}()
	if sf.iter == nil {
		sf.iter = fskit.NewDirIter(func() (entries []fs.DirEntry, err error) {
			return sf.sfs.ReadDir(sf.path)
		})
	}
	entries, err = sf.iter.ReadDir(n)
	return
}

// Close wraps the underlying Close and syncs modified files to remote.
// if local file gone after close, no need to sync.
// todo: if local file modtime newer than file, no need to sync?
func (sf *syncFile) Close() error {
	// Close the underlying file first
	err := sf.File.Close()

	// If file was modified, sync to remote
	if sf.modified && !sf.isDir {
		sf.sfs.changed(sf.path, true)
	}
	return err
}

// Readlink reads the target of a symbolic link
func (sfs *SyncFS) Readlink(name string) (string, error) {
	target, err := fs.Readlink(sfs.local, name)
	if err == nil {
		return target, nil
	}

	return "", err
}

// Symlink creates a symbolic link
func (sfs *SyncFS) Symlink(oldname, newname string) error {
	newname = sfs.clean(newname)
	oldname = sfs.clean(oldname)
	sfs.wait()
	defer sfs.changed(newname, true)
	sfs.log.Debug("Symlink", "oldname", oldname, "newname", newname)

	err := fs.Symlink(sfs.local, oldname, newname)
	if err != nil {
		return err
	}

	return nil
}
