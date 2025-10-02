package cachefs

import (
	"context"
	"fmt"
	"time"

	"tractor.dev/wanix/fs"
)

// Core filesystem interface implementations

// Open opens the named file for reading using the read-through cache
func (cfs *CacheFS) Open(name string) (fs.File, error) {
	return cfs.OpenContext(context.Background(), name)
}

// OpenContext opens the named file for reading with context using the read-through cache
func (cfs *CacheFS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	// Use readthru cache to get the file
	file, err := cfs.cache.Get(ctx, name, func(path string) (fs.File, error) {
		return fs.OpenContext(ctx, cfs.local, path)
	})
	if err != nil {
		return nil, err
	}

	// Wrap the file to track writes
	return &cacheFile{
		File:    file,
		cfs:     cfs,
		path:    name,
		isWrite: false,
	}, nil
}

// Create creates or truncates the named file
func (cfs *CacheFS) Create(name string) (fs.File, error) {
	return cfs.CreateContext(context.Background(), name)
}

// CreateContext creates or truncates the named file with context
func (cfs *CacheFS) CreateContext(ctx context.Context, name string) (fs.File, error) {
	// Ensure parent directory exists locally
	if err := cfs.ensureLocalParentDir(name); err != nil {
		return nil, fmt.Errorf("ensure local parent dir: %w", err)
	}

	// Create file locally using fs.Create
	localFile, err := fs.Create(cfs.local, name)
	if err != nil {
		return nil, fmt.Errorf("create local file: %w", err)
	}

	// Invalidate cache since we're creating a new file
	cfs.cache.Invalidate(name)

	return &cacheFile{
		File:     localFile,
		cfs:      cfs,
		path:     name,
		isWrite:  true,
		isCreate: true,
	}, nil
}

// Stat returns file information, checking cache first
func (cfs *CacheFS) Stat(name string) (fs.FileInfo, error) {
	return cfs.StatContext(context.Background(), name)
}

// StatContext returns file information with context, checking cache first
func (cfs *CacheFS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	// Check if we have valid cached metadata
	if cfs.cache.IsValid(name) {
		// Try to get info from local file
		if info, err := fs.StatContext(ctx, cfs.local, name); err == nil {
			return info, nil
		}
	}

	// Cache miss or invalid, get from remote
	return fs.StatContext(ctx, cfs.remote, name)
}

// Directory operations

// Mkdir creates a directory
func (cfs *CacheFS) Mkdir(name string, perm fs.FileMode) error {
	return cfs.MkdirContext(context.Background(), name, perm)
}

// MkdirContext creates a directory with context
func (cfs *CacheFS) MkdirContext(ctx context.Context, name string, perm fs.FileMode) error {
	// Create directory locally first using fs.Mkdir
	if err := fs.Mkdir(cfs.local, name, perm); err != nil {
		return fmt.Errorf("create local directory: %w", err)
	}

	// Invalidate cache for this path
	cfs.cache.Invalidate(name)

	// Async write to remote
	cfs.asyncWriteToRemote(name, []byte{}, &dirInfo{
		name:    name,
		mode:    perm | fs.ModeDir,
		modTime: time.Now(),
		isDir:   true,
	})

	return nil
}

// Remove removes a file or empty directory
func (cfs *CacheFS) Remove(name string) error {
	return cfs.RemoveContext(context.Background(), name)
}

// RemoveContext removes a file or directory with context
func (cfs *CacheFS) RemoveContext(ctx context.Context, name string) error {
	// Remove from local filesystem first using fs.Remove
	if err := fs.Remove(cfs.local, name); err != nil {
		return fmt.Errorf("remove from local: %w", err)
	}

	// Invalidate cache
	cfs.cache.Invalidate(name)

	// Async remove from remote
	cfs.pendingWrites.Add(1)
	go func() {
		defer cfs.pendingWrites.Done()

		if err := fs.Remove(cfs.remote, name); err != nil {
			select {
			case cfs.writeErrors <- fmt.Errorf("remove %s from remote: %w", name, err):
			default:
			}
		}
	}()

	return nil
}

// Metadata operations

// Chmod changes the mode of the named file
func (cfs *CacheFS) Chmod(name string, mode fs.FileMode) error {
	return cfs.ChmodContext(context.Background(), name, mode)
}

// ChmodContext changes file mode with context
func (cfs *CacheFS) ChmodContext(ctx context.Context, name string, mode fs.FileMode) error {
	// Change mode locally first using fs.Chmod
	if err := fs.Chmod(cfs.local, name, mode); err != nil {
		return fmt.Errorf("chmod local: %w", err)
	}

	// Invalidate cache
	cfs.cache.Invalidate(name)

	// Async chmod on remote
	cfs.pendingWrites.Add(1)
	go func() {
		defer cfs.pendingWrites.Done()

		if err := fs.Chmod(cfs.remote, name, mode); err != nil {
			select {
			case cfs.writeErrors <- fmt.Errorf("chmod %s on remote: %w", name, err):
			default:
			}
		}
	}()

	return nil
}

// Chown changes the numeric uid and gid of the named file
func (cfs *CacheFS) Chown(name string, uid, gid int) error {
	return cfs.ChownContext(context.Background(), name, uid, gid)
}

// ChownContext changes ownership with context
func (cfs *CacheFS) ChownContext(ctx context.Context, name string, uid, gid int) error {
	// Change ownership locally first using fs.Chown
	if err := fs.Chown(cfs.local, name, uid, gid); err != nil {
		return fmt.Errorf("chown local: %w", err)
	}

	// Invalidate cache
	cfs.cache.Invalidate(name)

	// Async chown on remote
	cfs.pendingWrites.Add(1)
	go func() {
		defer cfs.pendingWrites.Done()

		if err := fs.Chown(cfs.remote, name, uid, gid); err != nil {
			select {
			case cfs.writeErrors <- fmt.Errorf("chown %s on remote: %w", name, err):
			default:
			}
		}
	}()

	return nil
}

// Chtimes changes the access and modification times of the named file
func (cfs *CacheFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return cfs.ChtimesContext(context.Background(), name, atime, mtime)
}

// ChtimesContext changes times with context
func (cfs *CacheFS) ChtimesContext(ctx context.Context, name string, atime time.Time, mtime time.Time) error {
	// Change times locally first using fs.Chtimes
	if err := fs.Chtimes(cfs.local, name, atime, mtime); err != nil {
		return fmt.Errorf("chtimes local: %w", err)
	}

	// Invalidate cache
	cfs.cache.Invalidate(name)

	// Async chtimes on remote
	cfs.pendingWrites.Add(1)
	go func() {
		defer cfs.pendingWrites.Done()

		if err := fs.Chtimes(cfs.remote, name, atime, mtime); err != nil {
			select {
			case cfs.writeErrors <- fmt.Errorf("chtimes %s on remote: %w", name, err):
			default:
			}
		}
	}()

	return nil
}

// Advanced operations

// Rename renames (moves) oldname to newname
func (cfs *CacheFS) Rename(oldname, newname string) error {
	return cfs.RenameContext(context.Background(), oldname, newname)
}

// RenameContext renames with context
func (cfs *CacheFS) RenameContext(ctx context.Context, oldname, newname string) error {
	// Rename locally first using fs.Rename
	if err := fs.Rename(cfs.local, oldname, newname); err != nil {
		return fmt.Errorf("rename local: %w", err)
	}

	// Invalidate cache for both paths
	cfs.cache.Invalidate(oldname)
	cfs.cache.Invalidate(newname)

	// Async rename on remote
	cfs.pendingWrites.Add(1)
	go func() {
		defer cfs.pendingWrites.Done()

		if err := fs.Rename(cfs.remote, oldname, newname); err != nil {
			select {
			case cfs.writeErrors <- fmt.Errorf("rename %s to %s on remote: %w", oldname, newname, err):
			default:
			}
		}
	}()

	return nil
}

// Symlink creates a symbolic link
func (cfs *CacheFS) Symlink(oldname, newname string) error {
	return cfs.SymlinkContext(context.Background(), oldname, newname)
}

// SymlinkContext creates a symbolic link with context
func (cfs *CacheFS) SymlinkContext(ctx context.Context, oldname, newname string) error {
	// Create symlink locally first using fs.Symlink
	if err := fs.Symlink(cfs.local, oldname, newname); err != nil {
		return fmt.Errorf("symlink local: %w", err)
	}

	// Invalidate cache
	cfs.cache.Invalidate(newname)

	// Async symlink on remote
	cfs.pendingWrites.Add(1)
	go func() {
		defer cfs.pendingWrites.Done()

		if err := fs.Symlink(cfs.remote, oldname, newname); err != nil {
			select {
			case cfs.writeErrors <- fmt.Errorf("symlink %s to %s on remote: %w", oldname, newname, err):
			default:
			}
		}
	}()

	return nil
}

// Readlink reads the target of a symbolic link
func (cfs *CacheFS) Readlink(name string) (string, error) {
	return cfs.ReadlinkContext(context.Background(), name)
}

// ReadlinkContext reads symlink target with context
func (cfs *CacheFS) ReadlinkContext(ctx context.Context, name string) (string, error) {
	// Try local first if cached
	if cfs.cache.IsValid(name) {
		if target, err := fs.Readlink(cfs.local, name); err == nil {
			return target, nil
		}
	}

	// Fall back to remote using fs.Readlink
	return fs.Readlink(cfs.remote, name)
}

// dirInfo is a simple implementation of fs.FileInfo for directories
type dirInfo struct {
	name    string
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (di *dirInfo) Name() string       { return di.name }
func (di *dirInfo) Size() int64        { return 0 }
func (di *dirInfo) Mode() fs.FileMode  { return di.mode }
func (di *dirInfo) ModTime() time.Time { return di.modTime }
func (di *dirInfo) IsDir() bool        { return di.isDir }
func (di *dirInfo) Sys() interface{}   { return nil }
