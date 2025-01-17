package ns

import (
	"context"
	"path"
	"slices"
	"sort"
	"strings"

	"tractor.dev/wanix/fs"

	"tractor.dev/wanix/fs/fskit"
)

// FS represents a namespace with Plan9-style file and directory bindings
type FS struct {
	bindings map[string][]fileRef
}

func New() *FS {
	return &FS{bindings: make(map[string][]fileRef)}
}

// fileRef represents a reference to a name in a specific filesystem,
// possibly the root of the filesystem
type fileRef struct {
	fs   fs.FS
	path string
}

func (ref *fileRef) fileInfo(ctx context.Context, fname string) (*fskit.Node, error) {
	fi, err := fs.StatContext(ctx, ref.fs, ref.path)
	if err != nil {
		return nil, err
	}
	return fskit.RawNode(fi, fname), nil
}

// Bind adds a file or directory to the namespace. If specified, mode is "after" (default), "before", or "replace",
// which controls the order of the bindings.
func (ns *FS) Bind(src fs.FS, srcPath, dstPath, mode string) error {
	if !fs.ValidPath(srcPath) {
		return &fs.PathError{Op: "bind", Path: srcPath, Err: fs.ErrNotExist}
	}
	if !fs.ValidPath(dstPath) {
		return &fs.PathError{Op: "bind", Path: dstPath, Err: fs.ErrNotExist}
	}

	// Check srcPath
	file, err := src.Open(srcPath)
	if err != nil {
		return err
	}
	file.Close()

	ref := fileRef{fs: src, path: srcPath}
	switch mode {
	case "", "after":
		ns.bindings[dstPath] = append([]fileRef{ref}, ns.bindings[dstPath]...)
	case "before":
		ns.bindings[dstPath] = append(ns.bindings[dstPath], ref)
	case "replace":
		ns.bindings[dstPath] = []fileRef{ref}
	default:
		return &fs.PathError{Op: "bind", Path: mode, Err: fs.ErrInvalid}
	}
	return nil
}

func (ns *FS) Stat(name string) (fs.FileInfo, error) {
	return ns.StatContext(context.Background(), name)
}

func (ns *FS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}

	// we implement Stat to try and avoid using Open for Stat
	// since it involves calling Stat on all sub filesystem roots
	// which could lead to stack overflow when there is a cycle.

	if name == "." {
		return fskit.Entry(name, fs.ModeDir|0755), nil
	}

	file, err := fs.OpenContext(ctx, ns, name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}

// Open implements fs.FS interface
func (ns *FS) Open(name string) (fs.File, error) {
	return ns.OpenContext(context.Background(), name)
}

// OpenContext ...
func (ns *FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	var dir *fskit.Node
	var dirEntries []fs.DirEntry

	// Check direct bindings
	if refs, exists := ns.bindings[name]; exists {
		for _, ref := range refs {
			fi, err := fs.StatContext(ctx, ref.fs, ref.path)
			if err != nil {
				return nil, err
			}
			if fi.IsDir() {
				// directory binding, add entries
				if dir == nil {
					dir = fskit.RawNode(fi, name)
				}
				entries, err := fs.ReadDirContext(ctx, ref.fs, ref.path)
				if err != nil {
					return nil, err
				}
				for _, entry := range entries {
					ei, err := entry.Info()
					if err != nil {
						return nil, err
					}
					dirEntries = append(dirEntries, fskit.RawNode(ei))
				}
			} else {
				// file binding
				if file, err := fs.OpenContext(ctx, ref.fs, ref.path); err == nil {
					return file, nil
				}
				continue
			}

		}
	}

	// Check subpaths of bindings
	var sortedPaths []string
	for p := range ns.bindings {
		sortedPaths = append(sortedPaths, p)
	}
	sort.Slice(sortedPaths, func(i, j int) bool {
		return len(sortedPaths[i]) < len(sortedPaths[j])
	})
	for _, bindPath := range sortedPaths {
		refs := ns.bindings[bindPath]
		if strings.HasPrefix(name, bindPath) || bindPath == "." {
			relativePath := strings.TrimPrefix(name, bindPath)
			relativePath = strings.TrimPrefix(relativePath, "/")
			for _, ref := range refs {
				if ref.path != "." {
					relativePath = path.Join(ref.path, relativePath)
				}
				fi, err := fs.StatContext(ctx, ref.fs, relativePath)
				if err != nil {
					continue
				}
				if fi.IsDir() {
					// directory found in under dir binding
					if dir == nil {
						dir = fskit.RawNode(fi, name)
					}
					entries, err := fs.ReadDirContext(ctx, ref.fs, relativePath)
					if err != nil {
						return nil, err
					}
					for _, entry := range entries {
						ei, err := entry.Info()
						if err != nil {
							return nil, err
						}
						dirEntries = append(dirEntries, fskit.RawNode(ei))
					}
				} else {
					// file found in under dir binding
					if file, err := fs.OpenContext(ctx, ref.fs, relativePath); err == nil {
						return file, nil
					}
				}
			}
		}
	}

	// Synthesized parent directories
	var need = make(map[string]bool)
	if name == "." {
		for fname, refs := range ns.bindings {
			i := strings.Index(fname, "/")
			if i < 0 {
				if fname != "." {
					for _, ref := range refs {
						if info, err := ref.fileInfo(ctx, fname); err == nil {
							dirEntries = append(dirEntries, info)
						}
					}
				}
			} else {
				need[fname[:i]] = true
			}
		}
	} else {
		prefix := name + "/"
		for fname, refs := range ns.bindings {
			if strings.HasPrefix(fname, prefix) {
				felem := fname[len(prefix):]
				i := strings.Index(felem, "/")
				if i < 0 {
					for _, ref := range refs {
						if info, err := ref.fileInfo(ctx, fname); err == nil {
							dirEntries = append(dirEntries, info)
						}
					}
				} else {
					need[fname[len(prefix):len(prefix)+i]] = true
				}
			}
		}
		// If the name is not binding,
		// and there are no children of the name,
		// then the directory is treated as not existing.
		if dirEntries == nil && len(need) == 0 {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
	}
	for _, fi := range dirEntries {
		delete(need, fi.Name())
	}
	for name := range need {
		dirEntries = append(dirEntries, fskit.Entry(name, fs.ModeDir|0755))
	}
	slices.SortFunc(dirEntries, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	return fskit.DirFile(fskit.Entry(name, fs.ModeDir|0755), dirEntries...), nil
}
