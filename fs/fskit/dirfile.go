package fskit

import (
	"slices"
	"sort"
	"strings"

	"tractor.dev/wanix/fs"
)

// dirFile is a directory fs.File implementing fs.ReadDirFile
type dirFile struct {
	fs.FileInfo
	path string
	iter *DirIter
}

func DirFile(info *Node, entries ...fs.DirEntry) fs.File {
	// Create a copy of the node to avoid modifying the original
	nodeCopy := *info
	if !nodeCopy.IsDir() {
		nodeCopy.mode |= fs.ModeDir
	}
	// not sure a better place to do this,
	// but we'll filter entries starting with # to "hide" them
	entries = slices.DeleteFunc(entries, func(e fs.DirEntry) bool {
		return strings.HasPrefix(e.Name(), "#")
	})
	return &dirFile{
		FileInfo: &nodeCopy,
		path:     nodeCopy.path,
		iter: NewDirIter(func() ([]fs.DirEntry, error) {
			return removeDuplicatesAndSort(entries), nil
		}),
	}
}

func (d *dirFile) Stat() (fs.FileInfo, error) { return d, nil }
func (d *dirFile) Close() error               { return nil }

// GetUID and GetGID delegate to the embedded FileInfo if it exposes ownership.
func (d *dirFile) GetUID() int {
	type uidProvider interface{ GetUID() int }
	if p, ok := d.FileInfo.(uidProvider); ok {
		return p.GetUID()
	}
	return 0
}

func (d *dirFile) GetGID() int {
	type gidProvider interface{ GetGID() int }
	if p, ok := d.FileInfo.(gidProvider); ok {
		return p.GetGID()
	}
	return 0
}


func (d *dirFile) Read(b []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.path, Err: fs.ErrInvalid}
}

func (d *dirFile) ReadDir(count int) ([]fs.DirEntry, error) {
	return d.iter.ReadDir(count)
}

func removeDuplicatesAndSort(entries []fs.DirEntry) []fs.DirEntry {
	lastIndex := make(map[string]int)
	for i, item := range entries {
		lastIndex[item.Name()] = i
	}

	var result []fs.DirEntry
	seen := make(map[string]bool)
	for i := len(entries) - 1; i >= 0; i-- {
		item := entries[i]
		if lastIndex[item.Name()] == i && !seen[item.Name()] {
			result = append([]fs.DirEntry{item}, result...)
			seen[item.Name()] = true
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result
}
