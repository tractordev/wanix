package syncfs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

type OpType string

const (
	OpWrite       OpType = "write"
	OpCreate      OpType = "create"
	OpRename      OpType = "rename"
	OpMkdir       OpType = "mkdir"
	OpRemove      OpType = "remove"
	OpChmod       OpType = "chmod"
	OpChown       OpType = "chown"
	OpChtimes     OpType = "chtimes"
	OpSymlink     OpType = "symlink"
	OpBatchRemove OpType = "remove:batch"
)

// WriteOp represents a write operation to be processed by the write worker
type WriteOp struct {
	Type      OpType      // "create", "write", "mkdir", "remove", "rename", "chmod", "chown", "chtimes", "symlink"
	Path      string      // Target path
	Data      []byte      // Data for write operations
	Mode      fs.FileMode // File mode for creates and chmod
	Oldpath   string      // For rename operations (also used as target for symlink)
	Uid       int         // For chown operations
	Gid       int         // For chown operations
	Atime     time.Time   // For chtimes operations
	Mtime     time.Time   // For chtimes operations
	Batch     []WriteOp   // For batch operations
	Done      chan error  // Completion notification
	Err       error       // Error returned by operation
	QueuedAt  time.Time
	OpTime    time.Duration
	TotalTime time.Duration
	OnFinish  func(op *WriteOp)
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
	local  fs.FS // Local filesystem (fast access)
	remote fs.FS // Remote filesystem (slower, authoritative)

	remoteQueue chan WriteOp // Buffered channel for write operations
	writeLocks  sync.Map     // Path -> sync.WaitGroup

	log *slog.Logger
}

// New creates a new SyncFS with the given local and remote filesystems
func New(local fs.FS, remote fs.WatchFS) *SyncFS {
	sfs := &SyncFS{
		local:       local,
		remote:      remote,
		remoteQueue: make(chan WriteOp, 1024), // Buffered channel
		log:         slog.Default(),           // for now
	}
	go sfs.writeWorker()
	// go sfs.syncWorker()
	return sfs
}

func (sfs *SyncFS) syncWorker() {
	ctx := context.Background()
	events, err := fs.Watch(sfs.remote, ctx, "...")
	if err != nil {
		sfs.log.Error("Watch", "err", err)
		return
	}
	var (
		debounceTimer *time.Timer
		debounceMu    sync.Mutex
	)
	debounceSync := func() {
		debounceMu.Lock()
		defer debounceMu.Unlock()
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
			sfs.sync()
		})
	}
	debounceSync() // initial sync
	for event := range events {
		sfs.log.Debug("Watch", "event", event)
		debounceSync()
	}
}

func (sfs *SyncFS) sync() {
	sfs.log.Debug("Sync")
	sfs.acquireLock(".")
	err := fs.WalkDir(sfs.remote, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if err := fs.MkdirAll(sfs.local, path, info.Mode().Perm()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		sfs.releaseLock(".")
		sfs.log.Error("Sync:Walk", "err", err)
		return
	}
	sfs.releaseLock(".")
	sfs.syncDir(".", true)
}

func (sfs *SyncFS) syncDir(path string, recursive bool) {
	sfs.acquireLock(path)
	entries, err := fs.ReadDir(sfs.remote, path)
	if err != nil {
		sfs.releaseLock(path)
		sfs.log.Error("ReadDir:remote", "err", err, "path", path)
		return
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(path, entry.Name()))
		} else {
			go func() {
				if err := sfs.copyFileIfNewer(filepath.Join(path, entry.Name())); err != nil {
					sfs.log.Error("copyFileIfNewer", "err", err, "path", filepath.Join(path, entry.Name()))
				}
			}()
		}
	}
	entries, err = fs.ReadDir(sfs.local, path)
	if err != nil {
		sfs.releaseLock(path)
		sfs.log.Error("ReadDir:local", "err", err, "path", path)
		return
	}
	for _, entry := range entries {
		exists, err := fs.Exists(sfs.remote, filepath.Join(path, entry.Name()))
		if err != nil {
			sfs.releaseLock(path)
			sfs.log.Error("Exists", "err", err, "path", filepath.Join(path, entry.Name()))
			return
		}
		if !exists {
			sfs.log.Debug("Removing local file", "path", filepath.Join(path, entry.Name()))
			if err := fs.Remove(sfs.local, filepath.Join(path, entry.Name())); err != nil {
				sfs.releaseLock(path)
				sfs.log.Error("Remove", "err", err, "path", filepath.Join(path, entry.Name()))
				return
			}
		}
	}
	sfs.releaseLock(path)

	if recursive {
		for _, dir := range dirs {
			sfs.syncDir(dir, recursive)
		}
	}

}

func (sfs *SyncFS) cleanPath(path string) string {
	cleanPath := strings.TrimPrefix(filepath.Clean(path), "/")
	if cleanPath == "" {
		cleanPath = "."
	}
	return cleanPath
}

func (sfs *SyncFS) wait(name string) {
	wg, ok := sfs.writeLocks.Load(name)
	if !ok {
		return
	}
	wg.(*sync.WaitGroup).Wait()
}

func (sfs *SyncFS) writeWait(name string, dir bool) {
	sfs.wait(".")
	if dir {
		sfs.wait(filepath.Dir(name))
	}
	sfs.wait(name)
}

func (sfs *SyncFS) acquireLock(name string) {
	sfs.wait(name)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	sfs.writeLocks.Store(name, wg)
}

func (sfs *SyncFS) releaseLock(name string) {
	wg, ok := sfs.writeLocks.LoadAndDelete(name)
	if !ok {
		return
	}
	wg.(*sync.WaitGroup).Done()
}

// writeWorker processes write operations in order
func (sfs *SyncFS) writeWorker() {
	for op := range sfs.remoteQueue {
		err := sfs.performWrite(op)
		if err != nil {
			sfs.log.Error("writeWorker", "err", err, "op", op.Type, "path", op.Path)
		}
	}
}

// performWrite executes a single write operation on the remote filesystem
func (sfs *SyncFS) performWrite(op WriteOp) error {
	sfs.log.Debug("WriteOp:start", "op", op.Type, "path", op.Path)
	startTime := time.Now()
	defer func() {
		sfs.log.Debug("WriteOp:end", "op", op.Type, "path", op.Path, "dur", time.Since(startTime))
	}()
	switch op.Type {
	case "create":
		f, err := fs.Create(sfs.remote, op.Path)
		if err != nil {
			return err
		}
		return f.Close()
	case "write":
		return fs.WriteFile(sfs.remote, op.Path, op.Data, op.Mode)
	case "mkdir":
		return fs.Mkdir(sfs.remote, op.Path, op.Mode)
	case "remove":
		return fs.Remove(sfs.remote, op.Path)
	case "rename":
		return fs.Rename(sfs.remote, op.Oldpath, op.Path)
	case "chmod":
		return fs.Chmod(sfs.remote, op.Path, op.Mode)
	case "chown":
		return fs.Chown(sfs.remote, op.Path, op.Uid, op.Gid)
	case "chtimes":
		return fs.Chtimes(sfs.remote, op.Path, op.Atime, op.Mtime)
	case "symlink":
		return fs.Symlink(sfs.remote, op.Oldpath, op.Path)
	default:
		return fmt.Errorf("unknown write operation: %s", op.Type)
	}
}

// queueWrite queues a write operation and optionally waits for completion
func (sfs *SyncFS) queueWrite(op WriteOp) error {
	// sfs.log.Debug("WriteOp:queued", "op", op.Type, "path", op.Path)
	op.QueuedAt = time.Now()
	sfs.remoteQueue <- op
	return nil
}

// copyFileIfNewer copies a file from remote to local if it's newer or doesn't exist
func (sfs *SyncFS) copyFileIfNewer(path string) error {
	// Check if local file exists and get its info
	localInfo, localErr := fs.Stat(sfs.local, path)

	// Get remote file info
	remoteInfo, remoteErr := fs.Stat(sfs.remote, path)
	if remoteErr != nil {
		return remoteErr
	}

	// Copy if local doesn't exist or remote is newer
	if localErr != nil || remoteInfo.ModTime().After(localInfo.ModTime()) {
		sfs.log.Debug("CopyFile", "path", path)
		return fs.CopyFS(sfs.remote, path, sfs.local, path)
	}

	return nil
}

// Close shuts down the SyncFS and waits for all operations to complete
func (sfs *SyncFS) Close() error {
	sfs.writeLocks.Range(func(key, value interface{}) bool {
		value.(*sync.WaitGroup).Wait()
		return true
	})
	close(sfs.remoteQueue)
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
	name = sfs.cleanPath(name)
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
	name = sfs.cleanPath(name)
	defer func() {
		sfs.log.Debug("Stat", "name", name, "err", err, "notexists", os.IsNotExist(err))
	}()
	info, err = fs.StatContext(ctx, sfs.local, name)
	return
}

// ReadDir reads directory entries, with smart caching
func (sfs *SyncFS) ReadDir(name string) (entries []fs.DirEntry, err error) {
	name = sfs.cleanPath(name)
	defer func() {
		sfs.log.Debug("ReadDir", "name", name, "entries", len(entries), "err", err)
	}()
	entries, err = fs.ReadDir(sfs.local, name)
	return
}

// Write operations - all apply locally immediately and queue for remote

// Create creates a new file
func (sfs *SyncFS) Create(name string) (fs.File, error) {
	name = sfs.cleanPath(name)
	sfs.writeWait(name, true)
	sfs.log.Debug("Create", "name", name)

	f, err := fs.Create(sfs.local, name)
	if err != nil {
		return nil, err
	}

	sfs.queueWrite(WriteOp{
		Type: "create",
		Path: name,
	})

	return sfs.newSyncFile(f, name)
}

// OpenFile opens a file with flags
// TODO: move opencontext here and forward?
func (sfs *SyncFS) OpenFile(name string, flag int, perm fs.FileMode) (f fs.File, err error) {
	name = sfs.cleanPath(name)
	sfs.writeWait(name, true)
	defer func() {
		sfs.log.Debug("OpenFile", "name", name, "flag", flag, "perm", perm, "err", err)
	}()

	f, err = fs.OpenFile(sfs.local, name, flag, perm)
	if err != nil {
		return nil, err
	}

	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0 {
		if flag&os.O_CREATE != 0 {
			sfs.queueWrite(WriteOp{
				Type: "create",
				Path: name,
				Mode: perm,
			})
		}
	}

	return sfs.newSyncFile(f, name)
}

// Mkdir creates a directory
func (sfs *SyncFS) Mkdir(name string, perm fs.FileMode) error {
	name = sfs.cleanPath(name)
	sfs.writeWait(name, true)
	sfs.log.Debug("Mkdir", "name", name, "perm", perm)

	err := fs.Mkdir(sfs.local, name, perm)
	if err != nil {
		return err
	}

	sfs.queueWrite(WriteOp{
		Type: "mkdir",
		Path: name,
		Mode: perm,
	})

	return nil
}

func (sfs *SyncFS) Remove(name string) error {
	name = sfs.cleanPath(name)
	sfs.writeWait(name, true)
	sfs.log.Debug("Remove", "name", name)

	err := fs.Remove(sfs.local, name)
	if err != nil {
		return err
	}

	sfs.queueWrite(WriteOp{
		Type: "remove",
		Path: name,
	})

	return nil
}

func (sfs *SyncFS) Rename(oldname, newname string) error {
	newname = sfs.cleanPath(newname)
	oldname = sfs.cleanPath(oldname)
	sfs.writeWait(newname, true)
	sfs.log.Debug("Rename", "oldname", oldname, "newname", newname)

	err := fs.Rename(sfs.local, oldname, newname)
	if err != nil {
		return err
	}

	sfs.queueWrite(WriteOp{
		Type:    "rename",
		Oldpath: oldname,
		Path:    newname,
	})

	return nil
}

func (sfs *SyncFS) Chmod(name string, mode fs.FileMode) error {
	name = sfs.cleanPath(name)
	sfs.writeWait(name, false)
	sfs.log.Debug("Chmod", "name", name, "mode", mode)

	err := fs.Chmod(sfs.local, name, mode)
	if err != nil {
		return err
	}

	sfs.queueWrite(WriteOp{
		Type: "chmod",
		Path: name,
		Mode: mode,
	})

	return nil
}

func (sfs *SyncFS) Chown(name string, uid, gid int) error {
	name = sfs.cleanPath(name)
	sfs.writeWait(name, false)
	sfs.log.Debug("Chown", "name", name, "uid", uid, "gid", gid)

	err := fs.Chown(sfs.local, name, uid, gid)
	if err != nil {
		return err
	}

	sfs.queueWrite(WriteOp{
		Type: "chown",
		Path: name,
		Uid:  uid,
		Gid:  gid,
	})

	return nil
}

func (sfs *SyncFS) Chtimes(name string, atime, mtime time.Time) error {
	name = sfs.cleanPath(name)
	sfs.writeWait(name, false)
	sfs.log.Debug("Chtimes", "name", name, "atime", atime, "mtime", mtime)

	err := fs.Chtimes(sfs.local, name, atime, mtime)
	if err != nil {
		return err
	}

	sfs.queueWrite(WriteOp{
		Type:  "chtimes",
		Path:  name,
		Atime: atime,
		Mtime: mtime,
	})

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
		path:     sfs.cleanPath(path),
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
		// Read the entire file content to sync
		data, readErr := fs.ReadFile(sf.sfs.local, sf.path)
		if readErr != nil {
			return readErr
		}
		// Get file info for mode
		info, statErr := fs.Stat(sf.sfs.local, sf.path)
		if statErr != nil {
			return statErr
		}

		// Queue write operation for remote
		sf.sfs.queueWrite(WriteOp{
			Type: "write",
			Path: sf.path,
			Data: data,
			Mode: info.Mode(),
		})

		// Queue chtimes operation to set accurate modification time
		// Use the write time we captured, not the current time
		sf.sfs.queueWrite(WriteOp{
			Type:  "chtimes",
			Path:  sf.path,
			Atime: sf.writeTime, // Use write time for both atime and mtime
			Mtime: sf.writeTime,
		})
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
	newname = sfs.cleanPath(newname)
	oldname = sfs.cleanPath(oldname)
	sfs.writeWait(newname, true)
	sfs.log.Debug("Symlink", "oldname", oldname, "newname", newname)

	err := fs.Symlink(sfs.local, oldname, newname)
	if err != nil {
		return err
	}

	sfs.queueWrite(WriteOp{
		Type:    "symlink",
		Oldpath: oldname, // symlink target
		Path:    newname, // symlink path
	})

	return nil
}
