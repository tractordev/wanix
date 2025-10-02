package cachefs

import (
	"context"
	"fmt"
	"io"
	"log"
	"path"
	"sync"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/internal/readthru"
)

// CacheFS implements a read-through cache with write-back functionality.
// Reads are served from the local filesystem if cached and valid, otherwise
// fetched from the remote filesystem and cached locally.
// Writes go to both local (synchronously) and remote (asynchronously) filesystems.
type CacheFS struct {
	local  fs.FS // Local filesystem used as cache
	remote fs.FS // Remote filesystem
	cache  *readthru.Cache
	logger *log.Logger

	// Configuration
	cacheTTL time.Duration

	// Async write tracking
	pendingWrites sync.WaitGroup
	writeErrors   chan error
}

// New creates a new CacheFS with the given local and remote filesystems.
// The local filesystem serves as the cache, and the remote filesystem is the source of truth.
func New(local, remote fs.FS, cacheTTL time.Duration, logger *log.Logger) *CacheFS {
	if logger == nil {
		logger = log.Default()
	}

	cfs := &CacheFS{
		local:       local,
		remote:      remote,
		logger:      logger,
		cacheTTL:    cacheTTL,
		writeErrors: make(chan error, 100), // Buffer for async write errors
	}

	// Create readthru cache with fetch and store functions
	cfs.cache = readthru.New(
		cacheTTL,
		cfs.fetchFromRemote,
		cfs.storeToLocal,
	)

	// Start error logging goroutine
	go cfs.logWriteErrors()

	return cfs
}

// fetchFromRemote implements readthru.FetchFunc
func (cfs *CacheFS) fetchFromRemote(ctx context.Context, path string) ([]byte, fs.FileInfo, error) {
	// Open file from remote filesystem
	file, err := fs.OpenContext(ctx, cfs.remote, path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}

	// Read content if it's a regular file
	if !info.IsDir() {
		content, err := io.ReadAll(file)
		if err != nil {
			return nil, nil, err
		}
		return content, info, nil
	}

	// For directories, read the directory listing
	if dirFile, ok := file.(fs.ReadDirFile); ok {
		entries, err := dirFile.ReadDir(-1)
		if err != nil {
			return nil, nil, err
		}

		// Convert directory entries to a simple text format
		var content []byte
		for _, entry := range entries {
			entryInfo, err := entry.Info()
			if err != nil {
				continue
			}
			line := fmt.Sprintf("%s %d\n", entry.Name(), entryInfo.Mode())
			content = append(content, []byte(line)...)
		}
		return content, info, nil
	}

	return []byte{}, info, nil
}

// storeToLocal implements readthru.StoreFunc
func (cfs *CacheFS) storeToLocal(path string, content []byte) error {
	// Ensure parent directory exists
	if err := cfs.ensureLocalParentDir(path); err != nil {
		return fmt.Errorf("ensure parent dir: %w", err)
	}

	// Create/write file to local filesystem using fs.Create
	file, err := fs.Create(cfs.local, path)
	if err != nil {
		return fmt.Errorf("create local file: %w", err)
	}
	defer file.Close()

	if len(content) > 0 {
		_, err = fs.Write(file, content)
		if err != nil {
			return fmt.Errorf("write to local file: %w", err)
		}
	}

	return nil
}

// ensureLocalParentDir ensures the parent directory exists in the local filesystem
func (cfs *CacheFS) ensureLocalParentDir(filePath string) error {
	dir := path.Dir(filePath)
	if dir == "." || dir == "/" {
		return nil
	}

	// Check if directory already exists
	if exists, _ := fs.DirExists(cfs.local, dir); exists {
		return nil
	}

	// Use fs.MkdirAll which handles interface assertions and fallbacks
	return fs.MkdirAll(cfs.local, dir, 0755)
}

// logWriteErrors logs errors from async write operations
func (cfs *CacheFS) logWriteErrors() {
	for err := range cfs.writeErrors {
		cfs.logger.Printf("cachefs: async write error: %v", err)
	}
}

// asyncWriteToRemote performs an asynchronous write to the remote filesystem
func (cfs *CacheFS) asyncWriteToRemote(path string, content []byte, info fs.FileInfo) {
	cfs.pendingWrites.Add(1)
	go func() {
		defer cfs.pendingWrites.Done()

		if err := cfs.writeToRemote(context.Background(), path, content, info); err != nil {
			errMsg := fmt.Errorf("write %s to remote: %w", path, err)
			select {
			case cfs.writeErrors <- errMsg:
			default:
				// Channel full, drop error but log it directly
				cfs.logger.Printf("cachefs: async write error (channel full): %v", errMsg)
			}
		}
	}()
}

// writeToRemote writes content to the remote filesystem
func (cfs *CacheFS) writeToRemote(ctx context.Context, path string, content []byte, info fs.FileInfo) error {
	if info.IsDir() {
		// Use fs.Mkdir which handles interface assertions and fallbacks
		return fs.Mkdir(cfs.remote, path, info.Mode())
	}

	// Use fs.Create which handles interface assertions and fallbacks
	file, err := fs.Create(cfs.remote, path)
	if err != nil {
		return fmt.Errorf("create remote file: %w", err)
	}
	defer file.Close()

	if len(content) > 0 {
		_, err = fs.Write(file, content)
		if err != nil {
			return fmt.Errorf("write to remote file: %w", err)
		}
	}

	return nil
}

// Wait waits for all pending async writes to complete
func (cfs *CacheFS) Wait() {
	cfs.pendingWrites.Wait()
}

// Close closes the CacheFS and waits for pending operations
func (cfs *CacheFS) Close() error {
	cfs.pendingWrites.Wait()
	close(cfs.writeErrors)
	return nil
}
