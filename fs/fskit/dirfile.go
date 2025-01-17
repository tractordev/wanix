package fskit

import (
	"io"
	"slices"
	"strings"

	"tractor.dev/wanix/fs"
)

// dirFile is a directory fs.File implementing fs.ReadDirFile
type dirFile struct {
	fs.FileInfo
	path    string
	entries []fs.DirEntry
	offset  int
}

func DirFile(info *Node, entries ...fs.DirEntry) fs.File {
	if !info.IsDir() {
		info.mode |= fs.ModeDir
	}
	// not sure a better place to do this,
	// but we'll filter entries starting with # to "hide" them
	entries = slices.DeleteFunc(entries, func(e fs.DirEntry) bool {
		return strings.HasPrefix(e.Name(), "#")
	})
	return &dirFile{FileInfo: info, path: info.name, entries: removeDuplicates(entries)}
}

func (d *dirFile) Stat() (fs.FileInfo, error) { return d, nil }
func (d *dirFile) Close() error               { return nil }
func (d *dirFile) Read(b []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.path, Err: fs.ErrInvalid}
}

func (d *dirFile) ReadDir(count int) ([]fs.DirEntry, error) {
	n := len(d.entries) - d.offset
	if n == 0 && count > 0 {
		return nil, io.EOF
	}
	if count > 0 && n > count {
		n = count
	}
	list := make([]fs.DirEntry, n)
	for i := range list {
		list[i] = d.entries[d.offset+i]
	}
	d.offset += n
	return list, nil
}

func removeDuplicates(entries []fs.DirEntry) []fs.DirEntry {
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
	return result
}
