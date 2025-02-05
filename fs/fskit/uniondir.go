package fskit

import (
	"sort"

	"tractor.dev/wanix/fs"
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
	return DirFile(Entry(name, mode), entries...)
}
