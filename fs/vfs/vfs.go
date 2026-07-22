package vfs

import (
	"context"
	"errors"
	"log"
	"path"
	"slices"
	"strings"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/bind"
	"tractor.dev/wanix/fs/fskit"
)

type BindOption = fs.BindOption
type BindType = fs.BindType

const (
	BindAfter   = fs.BindAfter
	BindReplace = fs.BindReplace
	BindBefore  = fs.BindBefore
	BindName    = fs.BindNS
)

// BindAllocator is an interface that can be implemented by a filesystem
// to allocate a new filesystem for a binding.
//
// Deprecated: will be replaced with a different mechanism.
type BindAllocator interface {
	BindAllocFS(name string) (fs.FS, error)
}

// NS represents a namespace with Plan9-style file and directory bindings.
type NS struct {
	table *bind.Table
	ctx   context.Context
}

var (
	_ fs.FS            = (*NS)(nil)
	_ fs.RouteFS       = (*NS)(nil)
	_ fs.BindFS        = (*NS)(nil)
	_ fs.UnbindFS      = (*NS)(nil)
	_ fs.OpenContextFS = (*NS)(nil)
	_ fs.StatContextFS = (*NS)(nil)
)

func New(ctx context.Context) *NS {
	return &NS{
		table: bind.New(),
		ctx:   ctx,
	}
}

func (ns *NS) Clone(ctx context.Context) *NS {
	return &NS{
		table: ns.table.Clone(),
		ctx:   ctx,
	}
}

func (ns *NS) Context() context.Context {
	return ns.ctx
}

func (ns *NS) Route(ctx context.Context, name string) (fs.FS, string, error) {
	return ns.table.Route(ctx, ns, name)
}

func (ns *NS) UnbindAll() error {
	ns.table.UnbindAll(func(k string) bool {
		return len(k) > 0 && k[0] == '#'
	})
	return nil
}

func (ns *NS) Unbind(src fs.FS, srcPath, dstPath string) error {
	ctx := fs.WithOrigin(fs.ContextFor(ns), ns, dstPath, "unbind")
	return ns.table.Unbind(ctx, src, srcPath, dstPath)
}

// Bind adds a file or directory to the namespace.
// Only the first placement option is used. Default is BindAfter.
func (ns *NS) Bind(src fs.FS, srcPath, dstPath string, opts ...BindOption) error {
	ctx := fs.WithOrigin(fs.ContextFor(ns), ns, dstPath, "bind")
	return ns.table.Bind(ctx, src, srcPath, dstPath, opts...)
}

// Binds returns all fileinfo for bindings in a directory
func (ns *NS) Binds(name string) ([]fs.FileInfo, error) {
	return ns.table.Binds(name)
}

func (ns *NS) String() string {
	return ns.table.String()
}

func (ns *NS) Stat(name string) (fs.FileInfo, error) {
	ctx := fs.WithOrigin(ns.ctx, ns, name, "stat")
	return ns.StatContext(ctx, name)
}

func (ns *NS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}

	ctx = fs.WithOrigin(ctx, ns, name, "stat")

	if name == "." {
		return fskit.Entry(name, fs.ModeDir|0755), nil
	}

	// Check direct bindings since they don't get resolved by the resolver.
	// todo: again, if there is a direct binding by this name, it might also
	// exist as a subpath of another binding. so this is not correct.
	if refs, exists := ns.table.Snapshot()[name]; exists {
		for _, ref := range refs {
			fi, err := ref.FileInfo(path.Base(name))
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
func (ns *NS) Open(name string) (fs.File, error) {
	ctx := fs.WithOrigin(ns.ctx, ns, name, "open")
	return ns.OpenContext(ctx, name)
}

// OpenContext opens a path in the namespace.
//
// Directory unions are recursive: when multiple bindings share a bind point
// (e.g. two trees bound at "."), Open merges directory listings at every
// descendant path where those trees overlap (e.g. "bin" shows entries from
// both a/bin and b/bin). This differs from Plan 9, where union applies only
// at the bound name itself and does not synthesize merged views deeper in
// the tree. Route may return a single binding for writes; merged views
// are produced here in Open.
func (ns *NS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	ctx = fs.WithOrigin(ctx, ns, name, "open")

	b := ns.table.Snapshot()

	var dir *fskit.Node
	var dirEntries []fs.DirEntry
	var foundDir bool

	// Check direct bindings
	if refs, exists := b[name]; exists {
		for _, ref := range refs {
			if ref.Info.IsDir() {
				if dir == nil {
					dir = fskit.RawNode(ref.Info, name)
					foundDir = true
				}
				entries, err := fs.ReadDirContext(ctx, ref.FS, ref.Path)
				if err != nil {
					log.Println("readdir error:", err)
					return nil, err
				}
				// Keep entries lazy: calling entry.Info() here would
				// stat every child over the wire on every readdir (a
				// p9kit mount turns that into walk+getattr+clunk per
				// entry — the wanix `ls` storm). DirEntry.Name()/Type()
				// suffice for merge, dedup, and the readdir reply;
				// callers pay Info() only if they actually need it.
				dirEntries = append(dirEntries, entries...)
			} else {
				if file, err := fs.OpenContext(ctx, ref.FS, ref.Path); err == nil {
					return file, nil
				}
			}
		}
	}

	// Check subpaths of bindings
	var bindPaths []string
	for p := range b {
		bindPaths = append(bindPaths, p)
	}
	for _, bindPath := range fskit.MatchPaths(bindPaths, name) {
		for _, ref := range b[bindPath] {
			relativePath := path.Join(ref.Path, strings.Trim(strings.TrimPrefix(name, bindPath), "/"))
			fi, err := fs.StatContext(ctx, ref.FS, relativePath)
			if err != nil {
				continue
			}
			if fi.IsDir() {
				if dir == nil {
					dir = fskit.RawNode(fi, name)
					foundDir = true
				}
				entries, err := fs.ReadDirContext(ctx, ref.FS, relativePath)
				if err != nil {
					log.Println("readdir error:", err)
					return nil, err
				}
				// Lazy: see the note in the direct-binding branch above.
				dirEntries = append(dirEntries, entries...)
			} else {
				if file, err := fs.OpenContext(ctx, ref.FS, relativePath); err == nil {
					return file, nil
				}
			}
		}
	}

	// Synthesized parent directories
	var need = make(map[string]bool)
	if name == "." {
		for fname, refs := range b {
			i := strings.Index(fname, "/")
			if i < 0 {
				if fname != "." {
					for _, ref := range refs {
						if info, err := ref.FileInfo(fname); err == nil {
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
		for fname, refs := range b {
			if strings.HasPrefix(fname, prefix) {
				felem := fname[len(prefix):]
				i := strings.Index(felem, "/")
				if i < 0 {
					for _, ref := range refs {
						if info, err := ref.FileInfo(fname); err == nil {
							dirEntries = append(dirEntries, info)
						}
					}
				} else {
					need[fname[len(prefix):len(prefix)+i]] = true
				}
			}
		}
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
