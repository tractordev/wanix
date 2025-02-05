package fskit

import (
	"context"
	"path"
	"slices"
	"sort"
	"strings"

	"tractor.dev/wanix/fs"
)

type MapFS map[string]fs.FS

var _ fs.FS = MapFS(nil)

// Sub returns an [fs.FS] corresponding to the subtree rooted at fsys's dir.
//
// This operates the same as [fs.Sub] with some additional handling:
// - if dir is a direct path to a [fs.FS], it will return that FS
// - if dir is a subpath of a key in the map, it will return a [fs.SubdirFS]
func (fsys MapFS) Sub(dir string) (fs.FS, error) {
	if dir == "." {
		return fsys, nil
	}
	// check if dir is directly in map
	subfs, found := fsys[dir]
	if found {
		return subfs, nil
	}
	// check subpaths of map dirs
	var sortedKeys []string
	for p := range fsys {
		sortedKeys = append(sortedKeys, p)
	}
	sort.Slice(sortedKeys, func(i, j int) bool {
		// sort by length, longest first
		return len(sortedKeys[i]) > len(sortedKeys[j])
	})
	for _, key := range sortedKeys {
		if strings.HasPrefix(dir, key) {
			relativePath := strings.TrimPrefix(dir, key)
			relativePath = strings.TrimPrefix(relativePath, "/")
			return &fs.SubdirFS{Fsys: fsys[key], Dir: relativePath}, nil
		}
	}
	// if we get here, dir is not in map
	// TODO: or should this return a [fs.SubdirFS] to error when used?
	return nil, &fs.PathError{Op: "sub", Path: dir, Err: fs.ErrNotExist}
}

func (fsys MapFS) Stat(name string) (fs.FileInfo, error) {
	return fsys.StatContext(context.Background(), name)
}

func (fsys MapFS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}

	// we implement Stat to try and avoid using Open for Stat
	// since it involves calling Stat on all sub filesystem roots
	// which could lead to stack overflow when there is a cycle.

	if name == "." {
		return Entry(name, fs.ModeDir|0555), nil
	}

	subfs, found := fsys[name]
	if found {
		return fs.StatContext(ctx, subfs, ".")
	}

	file, err := fs.OpenContext(ctx, fsys, name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}

func (fsys MapFS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

func (fsys MapFS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	subfs, found := fsys[name]
	n, isNode := subfs.(*Node)
	if found && !isNode {
		namedFS := NamedFS(subfs, path.Base(name))
		return fs.OpenContext(ctx, namedFS, ".")
	}
	if found && isNode {
		subfs = NamedFS(subfs, path.Base(name))
		if !n.IsDir() {
			// Ordinary file
			return fs.OpenContext(ctx, subfs, ".")
		}
		// otherwise its a directory entry...
	}

	for p, subfs := range fsys {
		if strings.HasPrefix(name, p+"/") {
			subPath := strings.TrimPrefix(name, p+"/")
			mountPath := strings.TrimSuffix(name, "/"+subPath)
			namedFS := NamedFS(subfs, path.Base(mountPath))
			return fs.OpenContext(ctx, namedFS, subPath)
		}
	}

	// Directory, possibly synthesized.
	// Note that file can be nil here: the map need not contain explicit parent directories for all its files.
	// But file can also be non-nil, in case the user wants to set metadata for the directory explicitly.
	// Either way, we need to construct the list of children of this directory.
	var list []*Node
	// var elem string
	var need = make(map[string]bool)
	if name == "." {
		// elem = "."
		for fname, subfs := range fsys {
			i := strings.Index(fname, "/")
			if i < 0 {
				if fname != "." {
					fi, err := fs.StatContext(ctx, subfs, ".")
					if err != nil {
						continue
					}
					list = append(list, RawNode(fi, fname))
				}
			} else {
				need[fname[:i]] = true
			}
		}
	} else {
		// elem = name[strings.LastIndex(name, "/")+1:]
		prefix := name + "/"
		for fname, subfs := range fsys {
			fi, err := fs.StatContext(ctx, subfs, ".")
			if err != nil {
				continue
			}
			if strings.HasPrefix(fname, prefix) {
				felem := fname[len(prefix):]
				i := strings.Index(felem, "/")
				if i < 0 {
					list = append(list, RawNode(fi, felem))
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
		list = append(list, RawNode(name, fs.FileMode(fs.ModeDir|0555)))
	}
	slices.SortFunc(list, func(a, b *Node) int {
		return strings.Compare(a.name, b.name)
	})

	if n == nil {
		n = RawNode(name, fs.ModeDir|0555)
	} else {
		n.name = name
	}

	var entries []fs.DirEntry
	for _, nn := range list {
		entries = append(entries, nn)
	}
	return DirFile(n, entries...), nil
}
