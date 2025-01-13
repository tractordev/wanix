package fskit

import (
	"path"
	"slices"
	"strings"

	"tractor.dev/wanix/fs"
)

type MapFS map[string]fs.FS

var _ fs.FS = MapFS(nil)

// Open opens the named file.
func (fsys MapFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	subfs, found := fsys[name]
	n, isNode := subfs.(*N)
	if found && !isNode {
		return NamedFS(subfs, path.Base(name)).Open(".")
	}
	if found && isNode {
		subfs = NamedFS(subfs, path.Base(name))
		if !n.IsDir() {
			// Ordinary file
			return subfs.Open(".")
		}
		// otherwise its a directory entry...
	}

	for p, subfs := range fsys {
		if strings.HasPrefix(name, p+"/") {
			subPath := strings.TrimPrefix(name, p+"/")
			mountPath := strings.TrimSuffix(name, "/"+subPath)
			return NamedFS(subfs, path.Base(mountPath)).Open(subPath)
		}
	}

	// Directory, possibly synthesized.
	// Note that file can be nil here: the map need not contain explicit parent directories for all its files.
	// But file can also be non-nil, in case the user wants to set metadata for the directory explicitly.
	// Either way, we need to construct the list of children of this directory.
	var list []*N
	// var elem string
	var need = make(map[string]bool)
	if name == "." {
		// elem = "."
		for fname, subfs := range fsys {
			fi, err := fs.Stat(subfs, ".")
			if err != nil {
				continue
			}
			i := strings.Index(fname, "/")
			if i < 0 {
				if fname != "." {
					list = append(list, Node(fi, fname))
				}
			} else {
				need[fname[:i]] = true
			}
		}
	} else {
		// elem = name[strings.LastIndex(name, "/")+1:]
		prefix := name + "/"
		for fname, subfs := range fsys {
			fi, err := fs.Stat(subfs, ".")
			if err != nil {
				continue
			}
			if strings.HasPrefix(fname, prefix) {
				felem := fname[len(prefix):]
				i := strings.Index(felem, "/")
				if i < 0 {
					list = append(list, Node(fi, felem))
				} else {
					need[fname[len(prefix):len(prefix)+i]] = true
				}
			}
		}
		// If the directory name is not in the map,
		// and there are no children of the name in the map,
		// then the directory is treated as not existing.
		if n == nil && list == nil && len(need) == 0 {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
	}
	for _, fi := range list {
		delete(need, fi.name)
	}
	for name := range need {
		list = append(list, Node(name, fs.FileMode(fs.ModeDir|0555)))
	}
	slices.SortFunc(list, func(a, b *N) int {
		return strings.Compare(a.name, b.name)
	})

	if n == nil {
		n = Node(name, fs.ModeDir|0555)
	}

	var entries []fs.DirEntry
	for _, n := range list {
		entries = append(entries, n)
	}
	return DirFile(name, n.Mode(), entries...), nil
}
