package fskit

import (
	"context"
	"io/fs"
	"sort"
)

func UnionDir(name string, mode fs.FileMode, dirs ...fs.File) fs.File {
	entryMap := make(map[string]fs.DirEntry)
	for _, f := range dirs {
		rd, ok := f.(fs.ReadDirFile)
		if !ok {
			continue
		}
		e, err := rd.ReadDir(-1)
		if err != nil {
			continue
		}
		for _, entry := range e {
			entryMap[entry.Name()] = entry
		}
	}
	var entries []fs.DirEntry
	for _, entry := range entryMap {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	return DirFile(name, mode, entries...)
}

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
		file, err := fsys.Open(name)
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
			entries, err := fs.ReadDir(fsys, name)
			if err == nil && len(entries) > 0 {
				// It's an implicit directory
				isDir = true
				foundAny = true
				// Create a directory entry for each filesystem that has content under this path
				if file, err := fsys.Open(name); err == nil {
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
