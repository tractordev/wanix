package namespace

import (
	"context"
	"errors"
	"log"
	"path"
	"slices"
	"strings"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

type BindMode int

const (
	ModeAfter   BindMode = 1
	ModeReplace BindMode = 0
	ModeBefore  BindMode = -1
)

// FS represents a namespace with Plan9-style file and directory bindings.
// Todo: figure out how to make this thread safe. Tricky because ResolveFS
// can call back into the namespace.
type FS struct {
	bindings map[string][]bindTarget
	ctx      context.Context
}

// bindTarget represents a reference to a name in a specific filesystem,
// possibly the root of the filesystem.
type bindTarget struct {
	fs   fs.FS
	path string
	fi   fs.FileInfo
}

// fileInfo returns the latest file info for the binding with the given name
func (ref *bindTarget) fileInfo(ctx context.Context, fname string) (*fskit.Node, error) {
	fi, err := fs.StatContext(ctx, ref.fs, ref.path)
	if err != nil {
		return nil, err
	}
	return fskit.RawNode(fi, fname), nil
}

func New(ctx context.Context) *FS {
	fsys := &FS{
		bindings: make(map[string][]bindTarget),
	}
	fsys.ctx = ctx //fs.WithOrigin(ctx, fsys, "", "new")
	return fsys
}

func (ns *FS) Context() context.Context {
	return ns.ctx
}

func (ns *FS) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	// todo: if there is a direct binding by this name, it might also
	// exist as a subpath of another binding. so this is not correct.
	if refs, ok := ns.bindings[name]; ok {
		if len(refs) == 1 {
			// if there is a single binding, return it
			return refs[0].fs, refs[0].path, nil
		} else {
			if !fs.IsReadOnly(ctx) {
				for _, ref := range refs {
					// using CreateFS to find the first writable binding
					if _, ok := ref.fs.(fs.CreateFS); ok {
						return ref.fs, ref.path, nil
					}
				}
			}
			// return the namespace so it can union bindings
			return ns, name, nil
		}
	}

	// now check subpaths of bindings
	var bindPaths []string
	for p := range ns.bindings {
		bindPaths = append(bindPaths, p)
	}
	for _, bindPath := range fskit.MatchPaths(bindPaths, name) {
		refs := ns.bindings[bindPath]
		relativeName := strings.Trim(strings.TrimPrefix(name, bindPath), "/")
		var toStat []bindTarget

		// log.Println("resolve:", bindPath, relativeName, name)

		// first try to resolve the name with ResolveFS
		for _, ref := range refs {
			fullName := path.Join(ref.path, relativeName)
			if resolver, ok := ref.fs.(fs.ResolveFS); ok {
				rfsys, rname, err := resolver.ResolveFS(ctx, fullName)
				if err != nil {
					if errors.Is(err, fs.ErrNotExist) {
						// certainly does not have name
						continue
					}
					return rfsys, rname, err
				}
				if rname != fullName || !fs.Equal(rfsys, ref.fs) {
					// certainly does have name
					return rfsys, rname, nil
				}
			}
			// otherwise, we need to stat the name
			toStat = append(toStat, ref)
		}

		for _, ref := range toStat {
			fullName := path.Join(ref.path, relativeName)
			// log.Println("resolve stat:", reflect.TypeOf(ref.fs), fullName)
			_, err := fs.StatContext(ctx, ref.fs, fullName)
			if err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					log.Println("resolve stat:", err)
				}
				continue
			}
			return ref.fs, fullName, nil
		}

		if slices.Contains([]string{"create", "mkdir", "symlink"}, fs.Op(ctx)) {
			// could be a new file (create, mkdir, etc), so check the directory
			for _, ref := range toStat {
				fullName := path.Join(ref.path, relativeName)
				_, err := fs.StatContext(ctx, ref.fs, path.Dir(fullName))
				if err != nil {
					continue
				}
				return ref.fs, fullName, nil
			}
		}
	}

	return ns, name, nil
}

func (ns *FS) Unbind(src fs.FS, srcPath, dstPath string) error {
	if !fs.ValidPath(srcPath) {
		return &fs.PathError{Op: "unbind", Path: srcPath, Err: fs.ErrNotExist}
	}
	if !fs.ValidPath(dstPath) {
		return &fs.PathError{Op: "unbind", Path: dstPath, Err: fs.ErrNotExist}
	}

	// Resolve the source path first, just like in Bind
	rfsys, rname, err := fs.Resolve(src, fs.ContextFor(ns), srcPath)
	if err != nil {
		return err
	}

	ns.bindings[dstPath] = slices.DeleteFunc(ns.bindings[dstPath], func(ref bindTarget) bool {
		return fs.Equal(ref.fs, rfsys) && ref.path == rname
	})
	if len(ns.bindings[dstPath]) == 0 {
		delete(ns.bindings, dstPath)
	}

	return nil
}

// Bind adds a file or directory to the namespace. If specified, mode is "after" (default), "before", or "replace",
// which controls the order of the bindings.
// TODO: replace mode arg with BindMode enum
func (ns *FS) Bind(src fs.FS, srcPath, dstPath, mode string) error {
	if !fs.ValidPath(srcPath) {
		return &fs.PathError{Op: "bind", Path: srcPath, Err: fs.ErrNotExist}
	}
	if !fs.ValidPath(dstPath) {
		return &fs.PathError{Op: "bind", Path: dstPath, Err: fs.ErrNotExist}
	}

	// Check srcPath, cache the file info
	rfsys, rname, err := fs.Resolve(src, fs.ContextFor(ns), srcPath)
	if err != nil {
		return err
	}
	file, err := rfsys.Open(rname)
	if err != nil {
		return err
	}
	fi, err := file.Stat()
	if err != nil {
		return err
	}
	file.Close()

	ref := bindTarget{fs: rfsys, path: rname, fi: fi}
	switch mode {
	case "", "after":
		ns.bindings[dstPath] = append([]bindTarget{ref}, ns.bindings[dstPath]...)
	case "before":
		ns.bindings[dstPath] = append(ns.bindings[dstPath], ref)
	case "replace":
		ns.bindings[dstPath] = []bindTarget{ref}
	default:
		return &fs.PathError{Op: "bind", Path: mode, Err: fs.ErrInvalid}
	}
	return nil
}

func (ns *FS) Stat(name string) (fs.FileInfo, error) {
	ctx := fs.WithOrigin(ns.ctx, ns, name, "stat")
	return ns.StatContext(ctx, name)
}

func (ns *FS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}

	ctx = fs.WithOrigin(ctx, ns, name, "stat")

	// we implement Stat to try and avoid using Open for Stat
	// since it involves calling Stat on all sub filesystem roots
	// which could lead to stack overflow when there is a cycle.

	if name == "." {
		return fskit.Entry(name, fs.ModeDir|0755), nil
	}

	// Check direct bindings since they don't get resolved by the resolver.
	// todo: again, if there is a direct binding by this name, it might also
	// exist as a subpath of another binding. so this is not correct.
	if refs, exists := ns.bindings[name]; exists {
		for _, ref := range refs {
			fi, err := ref.fileInfo(ctx, path.Base(name))
			if err != nil {
				continue
			}
			return fi, nil
		}
	}

	tfsys, tname, err := fs.ResolveTo[fs.StatContextFS](ns, ctx, name)
	if err != nil && !errors.Is(err, fs.ErrNotSupported) {
		return nil, err
	}
	if err == nil && !fs.Equal(tfsys, ns) {
		return tfsys.StatContext(ctx, tname)
	}

	rfsys, rname, err := fs.Resolve(ns, ctx, name)
	if err != nil {
		return nil, err
	}

	f, err := fs.OpenContext(ctx, rfsys, rname)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
}

// Open implements fs.FS interface
func (ns *FS) Open(name string) (fs.File, error) {
	ctx := fs.WithOrigin(ns.ctx, ns, name, "open")
	return ns.OpenContext(ctx, name)
}

// OpenContext ...
func (ns *FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	ctx = fs.WithOrigin(ctx, ns, name, "open")

	var dir *fskit.Node
	var dirEntries []fs.DirEntry
	var foundDir bool

	// Check direct bindings
	if refs, exists := ns.bindings[name]; exists {
		for _, ref := range refs {
			if ref.fi.IsDir() {
				// directory binding, add entries
				if dir == nil {
					dir = fskit.RawNode(ref.fi, name)
					foundDir = true
				}
				entries, err := fs.ReadDirContext(ctx, ref.fs, ref.path)
				if err != nil {
					log.Println("readdir error:", err)
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
	var bindPaths []string
	for p := range ns.bindings {
		bindPaths = append(bindPaths, p)
	}
	for _, bindPath := range fskit.MatchPaths(bindPaths, name) {
		for _, ref := range ns.bindings[bindPath] {
			relativePath := path.Join(ref.path, strings.Trim(strings.TrimPrefix(name, bindPath), "/"))
			fi, err := fs.StatContext(ctx, ref.fs, relativePath)
			if err != nil {
				continue
			}
			if fi.IsDir() {
				// directory found in under dir binding
				if dir == nil {
					dir = fskit.RawNode(fi, name)
					foundDir = true
				}
				entries, err := fs.ReadDirContext(ctx, ref.fs, relativePath)
				if err != nil {
					log.Println("readdir error:", err)
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
		// and there are no children of the name and no dir was found,
		// then the directory is treated as not existing.
		if dirEntries == nil && len(need) == 0 && !foundDir {
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
