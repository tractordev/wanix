package cachefs

import (
	"bytes"
	"context"
	"io"

	"tractor.dev/wanix/fs"
)

// cacheFile wraps a file from the local filesystem and handles write-back to remote
type cacheFile struct {
	fs.File
	cfs      *CacheFS
	path     string
	isWrite  bool
	isCreate bool
	buffer   *bytes.Buffer // Buffer to capture writes for async remote write
}

// Write writes data to the local file and buffers it for async remote write
func (cf *cacheFile) Write(p []byte) (int, error) {
	// Write to the underlying local file
	n, err := fs.Write(cf.File, p)
	if err != nil {
		return n, err
	}

	// Buffer the write for async remote write
	if cf.buffer == nil {
		cf.buffer = &bytes.Buffer{}
	}
	cf.buffer.Write(p[:n])
	cf.isWrite = true

	return n, nil
}

// WriteAt writes data at the specified offset
func (cf *cacheFile) WriteAt(p []byte, off int64) (int, error) {
	// Write to the underlying local file
	n, err := fs.WriteAt(cf.File, p, off)
	if err != nil {
		return n, err
	}

	// For WriteAt, we need to track that this is a write operation
	// but we can't easily buffer partial writes, so we'll read the entire
	// file content when closing to send to remote
	cf.isWrite = true

	return n, nil
}

// Close closes the file and triggers async write to remote if there were writes
func (cf *cacheFile) Close() error {
	// Close the underlying file first
	err := cf.File.Close()

	// If this was a write operation or a create operation, trigger async write to remote
	if cf.isWrite || cf.isCreate {
		cf.cfs.cache.Invalidate(cf.path)

		// Get file info for the async write
		info, statErr := fs.StatContext(context.TODO(), cf.cfs.local, cf.path)
		if statErr != nil {
			// Log error but don't fail the close
			select {
			case cf.cfs.writeErrors <- statErr:
			default:
			}
			return err
		}

		// Read the entire file content for async write to remote
		content, readErr := fs.ReadFile(cf.cfs.local, cf.path)
		if readErr != nil {
			// Log error but don't fail the close
			select {
			case cf.cfs.writeErrors <- readErr:
			default:
			}
			return err
		}

		// Trigger async write to remote
		cf.cfs.asyncWriteToRemote(cf.path, content, info)
	}

	return err
}

// Seek delegates to the underlying file
func (cf *cacheFile) Seek(offset int64, whence int) (int64, error) {
	if seeker, ok := cf.File.(io.Seeker); ok {
		return seeker.Seek(offset, whence)
	}
	return 0, fs.ErrNotSupported
}

// ReadDir delegates to the underlying file for directory operations
func (cf *cacheFile) ReadDir(count int) ([]fs.DirEntry, error) {
	if dirFile, ok := cf.File.(fs.ReadDirFile); ok {
		return dirFile.ReadDir(count)
	}
	return nil, fs.ErrNotSupported
}

// Sync ensures data is written to storage (delegates to underlying file)
func (cf *cacheFile) Sync() error {
	if syncFile, ok := cf.File.(interface{ Sync() error }); ok {
		return syncFile.Sync()
	}
	return nil // No-op if not supported
}
