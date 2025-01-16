package fskit

import (
	"context"
	"path"
	"slices"
	"strings"

	"tractor.dev/wanix/fs"
)

type MapFS map[string]fs.FS

var _ fs.FS = MapFS(nil)

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
			fi, err := fs.StatContext(ctx, subfs, ".")
			if err != nil {
				continue
			}
			i := strings.Index(fname, "/")
			if i < 0 {
				if fname != "." {
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
