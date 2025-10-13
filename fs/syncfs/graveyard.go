//go:build graveyard

package syncfs

import (
	"fmt"
	"path/filepath"
	"time"

	"tractor.dev/wanix/fs"
)

// CachedDir represents cached information about a remote directory
type CachedDir struct {
	entries   []fs.DirEntry // Cached directory entries
	expiresAt time.Time     // When this cache expires
	renewsAt  time.Time     // When this cache renews
}

// ReadDirSync synchronizes directory entries from remote to local cache and returns entries.
// Called for SyncAll and ReadDir on cache miss.
// todo: cancel sync?
func (sfs *SyncFS) ReadDirSync(path string) (entries []fs.DirEntry, err error) {
	path = sfs.cleanPath(path)
	defer func() {
		sfs.log.Debug("ReadDirSync", "path", path, "entries", len(entries), "err", err)
	}()

	entries, err = fs.ReadDir(sfs.remote, path)
	if err != nil {
		return nil, fmt.Errorf("pull ReadDir failed for path %q: %w", path, err)
	}

	// Update cache using cleaned path
	sfs.cacheMu.Lock()
	sfs.dirCache[path] = &CachedDir{
		entries:   entries,
		expiresAt: time.Now().Add(sfs.defaultExpiry),
		renewsAt:  time.Now().Add(sfs.defaultExpiry / 2),
	}
	sfs.cacheMu.Unlock()

	return entries, nil
}

// SyncNode performs breadth-first synchronization of a directory subtree
// todo: check local first. after sync, find whats in local not in remote, remove.
func (sfs *SyncFS) SyncNode(path string) error {
	path = sfs.cleanPath(path)
	sfs.log.Debug("SyncNode", "path", path)

	if path != "." {
		dir := filepath.Dir(path)
		if err := sfs.SyncDir(dir); err != nil {
			return err
		}
	}

	entries, err := sfs.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if err := sfs.SyncDir(filepath.Join(path, entry.Name())); err != nil {
				return err
			}
		}
	}

	return nil
}

func (sfs *SyncFS) SyncDir(path string) error {
	path = sfs.cleanPath(path)
	sfs.log.Debug("SyncDir", "path", path)

	entries, err := sfs.ReadDirSync(path)
	if err != nil {
		return err
	}

	// Process entries breadth-first
	// queue := make([]string, 0)

	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		fmt.Println("SyncDir", "entryPath", entryPath)

		sfs.cacheMu.Lock()
		_, exists := sfs.tombstones[entryPath]
		sfs.cacheMu.Unlock()
		if exists {
			continue
		}

		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		// Ensure directory exists locally
		if err := fs.MkdirAll(sfs.local, filepath.Dir(entryPath), entryInfo.Mode().Perm()); err != nil {
			return err
		}

		if entry.IsDir() {
			if err := fs.MkdirAll(sfs.local, entryPath, entryInfo.Mode().Perm()); err != nil {
				return err
			}
			// Queue for next level processing
			// queue = append(queue, entryPath)
		} else {
			go func() {
				// Copy file if needed
				if err := sfs.copyFileIfNewer(entryPath); err != nil {
					sfs.log.Error("copyFileIfNewer", "err", err, "path", entryPath)
				}
			}()
			return nil
		}
	}

	// Process queued directories
	// for _, dirPath := range queue {
	// 	select {
	// 	case <-sfs.ctx.Done():
	// 		return
	// 	default:
	// 		// Pull subdirectory (this will recursively sync)
	// 		go sfs.SyncAll(dirPath)
	// 	}
	// }
	return nil

}

// isExpired checks if a cache entry has expired
func (sfs *SyncFS) isExpired(entry *CachedDir) bool {
	return time.Now().After(entry.expiresAt)
}

// isRenewable checks if a cache entry can be renewed
func (sfs *SyncFS) isRenewable(entry *CachedDir) bool {
	return time.Now().After(entry.renewsAt)
}

// CachedReadDir returns cached entries if they exist and haven't expired
func (sfs *SyncFS) CachedReadDir(path string) (entries []fs.DirEntry, found bool, pending bool) {
	defer func() {
		sfs.log.Debug("CachedReadDir", "path", path, "entries", len(entries), "found", found, "pending", pending)
	}()
	sfs.cacheMu.RLock()

	path = sfs.cleanPath(path)
	entry, exists := sfs.dirCache[path]
	if !exists {
		sfs.log.Debug("CachedReadDir miss", "path", path)
		sfs.cacheMu.RUnlock()
		return nil, false, false
	}
	if sfs.isExpired(entry) {
		// if sfs.isPending(entry) {
		// 	sfs.log.Debug("CachedReadDir expired pending", "path", path, "pending", entry.pending)
		// 	sfs.cacheMu.RUnlock()
		// 	return nil, false, true
		// }
		sfs.log.Debug("CachedReadDir expired", "path", path)
		sfs.cacheMu.RUnlock()
		sfs.cacheMu.Lock()
		delete(sfs.dirCache, path)
		sfs.cacheMu.Unlock()
		return nil, false, false
	}

	if sfs.isRenewable(entry) {
		go sfs.SyncDir(path)
	}

	defer sfs.cacheMu.RUnlock()
	sfs.log.Debug("CachedReadDir hit", "path", path, "renew", sfs.isRenewable(entry))
	return entry.entries, true, false
}

func (op *WriteOp) RequiresDrain() bool {
	return op.Type == OpRemove || op.Type == OpRename || op.Type == OpBatchRemove
}

func (op *WriteOp) CanBatch() bool {
	return op.Type == OpRemove
}

func (op *WriteOp) AffectedPaths() []string {
	if op.Type == OpRename {
		return []string{op.Path, op.Oldpath}
	}
	return []string{op.Path}
}

func (op *WriteOp) Finish(err error) {
	op.Err = err
	if !op.QueuedAt.IsZero() {
		op.TotalTime = time.Since(op.QueuedAt)
	}
	if op.OnFinish != nil {
		op.OnFinish(op)
	}
	if op.Done != nil {
		op.Done <- err
	}
}

func (sfs *SyncFS) pendingWritesIncr(dirpath string, n int) bool {
	dirpath = sfs.cleanPath(dirpath)
	parentdir := filepath.Dir(dirpath)

	var parent *CachedDir
	var parentExists bool
	var existNotice bool
	for !parentExists {
		sfs.cacheMu.Lock()
		parent, parentExists = sfs.dirCache[parentdir]
		sfs.cacheMu.Unlock()
		if parentExists {
			break
		}
		if !existNotice {
			existNotice = true
			sfs.log.Debug("WAITING FOR CACHE FOR PARENT", "path", dirpath, "parent", parentdir)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if existNotice {
		sfs.log.Debug("OK: FOUND CACHE FOR PARENT", "path", dirpath, "parent", parentdir)
	}

	var parentPending int = -1
	var pendingNotice bool
	for parentPending != 0 {
		sfs.cacheMu.Lock()
		parent, parentExists = sfs.dirCache[parentdir]
		sfs.cacheMu.Unlock()
		if !parentExists {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		parent.mu.Lock()
		parentPending = parent.pending
		parent.mu.Unlock()
		if parentPending == 0 {
			break
		}
		if !pendingNotice {
			pendingNotice = true
			sfs.log.Debug("WAITING FOR PENDING PARENT", "path", dirpath, "parent", parentdir, "pending", parentPending)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if pendingNotice {
		sfs.log.Debug("OK: PARENT NO LONGER PENDING", "path", dirpath, "parent", parentdir, "pending", parentPending)
	}

	// Wait for dirCache[dirpath] to exist before proceeding
	var entry *CachedDir
	var entryExists bool
	var entryExistNotice bool
	for !entryExists {
		sfs.cacheMu.Lock()
		entry, entryExists = sfs.dirCache[dirpath]
		sfs.cacheMu.Unlock()
		if entryExists {
			break
		}
		if !entryExistNotice {
			entryExistNotice = true
			sfs.log.Debug("WAITING FOR CACHE FOR DIR", "path", dirpath)
		}
		time.Sleep(100 * time.Millisecond)
		_, err := sfs.ReadDirSync(dirpath)
		if err != nil {
			sfs.log.Error("ERROR: READING DIR", "path", dirpath, "err", err)
			return false
		}
	}
	if entryExistNotice {
		sfs.log.Debug("OK: FOUND CACHE FOR DIR", "path", dirpath)
	}

	sfs.cacheMu.Lock()
	defer sfs.cacheMu.Unlock()
	entry, exists := sfs.dirCache[dirpath]
	if exists {
		entry.mu.Lock()
		entry.expiresAt = time.Now()
		entry.pending += n
		if entry.pending < 0 {
			entry.pending = 0
		}
		pending := entry.pending
		entry.mu.Unlock()
		if pending == 0 {
			go sfs.SyncDir(dirpath)
			sfs.log.Debug("WritesComplete", "path", dirpath)

			// 	// When pending operations complete, delay cache expiry slightly
			// 	// to avoid race condition where Pull returns stale data
			// 	entry.expiresAt = time.Now().Add(500 * time.Millisecond)
		}

	} else {
		sfs.log.Error("!! NO CACHE FOR PENDING", "path", dirpath, "n", n)
	}
	return true
}
