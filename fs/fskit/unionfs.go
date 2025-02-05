package fskit

import (
	"context"

	"tractor.dev/wanix/fs"
)

// read-only union of filesystems
type UnionFS []fs.FS

func (f UnionFS) Open(name string) (fs.File, error) {
	return f.OpenContext(context.Background(), name)
}

func (f UnionFS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	var files []fs.File
	var isDir bool
	var foundAny bool

	// First check if it exists as a file/dir in any filesystem
	for _, fsys := range f {
		file, err := fs.OpenContext(ctx, fsys, name)
		if err != nil {
			continue
		}
		info, err := file.Stat()
		if err != nil {
			file.Close()
			continue
		}
		if foundAny && info.IsDir() != isDir {
			file.Close()
			continue
		}
		foundAny = true
		isDir = info.IsDir()
		files = append(files, file)
	}

	// If not found directly, check if it might be an implicit directory
	if !foundAny && name != "." {
		// Check if any filesystem has files under this path
		for _, fsys := range f {
			entries, err := fs.ReadDirContext(ctx, fsys, name)
			if err == nil && len(entries) > 0 {
				// It's an implicit directory
				isDir = true
				foundAny = true
				// Create a directory entry for each filesystem that has content under this path
				if file, err := fs.OpenContext(ctx, fsys, name); err == nil {
					files = append(files, file)
				}
			}
		}
	}

	if !foundAny {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	if isDir {
		return UnionDir(name, 0555, files...), nil
	}
	return files[0], nil
}

func (f UnionFS) ReadDir(name string) ([]fs.DirEntry, error) {
	file, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if dir, ok := file.(fs.ReadDirFile); ok {
		return dir.ReadDir(0)
	}
	return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
}

// Sub returns an [fs.FS] corresponding to the subtree rooted at fsys's dir.
//
// If dir is ".", Sub returns fsys unchanged.
// If only one filesystem in the union contains dir, Sub returns that filesystem's subtree directly.
// Otherwise, Sub returns a new UnionFS containing all valid subtrees.
func (f UnionFS) Sub(dir string) (fs.FS, error) {
	if !fs.ValidPath(dir) {
		return nil, &fs.PathError{Op: "sub", Path: dir, Err: fs.ErrInvalid}
	}
	if dir == "." {
		return f, nil
	}

	// Collect all valid sub-filesystems
	var subFs []fs.FS
	for _, fsys := range f {
		if sub, err := fs.Sub(fsys, dir); err == nil {
			subFs = append(subFs, sub)
		}
	}

	if len(subFs) == 0 {
		return nil, &fs.PathError{Op: "sub", Path: dir, Err: fs.ErrNotExist}
	}

	// If only one filesystem has this directory, return it directly
	if len(subFs) == 1 {
		return subFs[0], nil
	}

	// Otherwise return a union of all sub-filesystems
	return UnionFS(subFs), nil
}
