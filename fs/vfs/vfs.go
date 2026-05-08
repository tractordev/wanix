package vfs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

type BindMode int

const (
	ModeAfter   BindMode = 1
	ModeReplace BindMode = 0
	ModeBefore  BindMode = -1
)

// BindAllocator is an interface that can be implemented by a filesystem
// to allocate a new filesystem for a binding.
type BindAllocator interface {
	// BindAllocFS is called when a new binding is added to the namespace.
	// It should return a new filesystem for the binding.
	// The name is the source path of the binding.
	BindAllocFS(name string) (fs.FS, error)
}

// NS represents a namespace with Plan9-style file and directory bindings.
//
// Concurrency model: copy-on-write. The bindings map lives behind an
// atomic.Pointer; reads grab a snapshot lock-free and may freely call out
// to other filesystems (including ones that recurse back into this NS via
// ResolveFS) without risking deadlock or writer starvation. Writers
// (Bind/Unbind/UnbindAll) serialize via writeMu, copy the current map,
// mutate the copy, and atomically swap it in.
type NS struct {
	bindings atomic.Pointer[map[string][]bindTarget]
	writeMu  sync.Mutex
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
	// Use the file info captured at bind time. Re-statting every bind target
	// during namespace directory synthesis can create recursive/stat storms
	// against remote filesystems (notably p9 guest mounts), which can starve
	// in-flight directory reads.
	return fskit.RawNode(ref.fi, fname), nil
}

func New(ctx context.Context) *NS {
	fsys := &NS{ctx: ctx}
	m := make(map[string][]bindTarget)
	fsys.bindings.Store(&m)
	return fsys
}

// snapshot returns the current bindings map. Callers MUST treat it as
// read-only -- the returned map and its slices may be shared with other
// readers and with prior snapshots.
func (ns *NS) snapshot() map[string][]bindTarget {
	return *ns.bindings.Load()
}

// mutate runs fn against a private copy of the bindings map, then atomically
// publishes that copy. Writers serialize on writeMu; readers continue to see
// the previous snapshot while fn is running.
func (ns *NS) mutate(fn func(m map[string][]bindTarget)) {
	ns.writeMu.Lock()
	defer ns.writeMu.Unlock()

	cur := *ns.bindings.Load()
	cp := make(map[string][]bindTarget, len(cur))
	for k, v := range cur {
		cp[k] = slices.Clone(v)
	}
	fn(cp)
	ns.bindings.Store(&cp)
}

func (ns *NS) Clone(ctx context.Context) *NS {
	cur := ns.snapshot()
	b := make(map[string][]bindTarget, len(cur))
	for k, v := range cur {
		b[k] = slices.Clone(v)
	}
	out := &NS{ctx: ctx}
	out.bindings.Store(&b)
	return out
}

func (ns *NS) Context() context.Context {
	return ns.ctx
}

func (ns *NS) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	b := ns.snapshot()

	// todo: if there is a direct binding by this name, it might also
	// exist as a subpath of another binding. so this is not correct.
	if refs, ok := b[name]; ok {
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
	for p := range b {
		bindPaths = append(bindPaths, p)
	}
	for _, bindPath := range fskit.MatchPaths(bindPaths, name) {
		refs := b[bindPath]
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
			_, err := fs.LstatContext(ctx, ref.fs, fullName)
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

func (ns *NS) UnbindAll() error {
	ns.mutate(func(m map[string][]bindTarget) {
		// all but special bindings for now
		for k := range m {
			if len(k) == 0 || k[0] != '#' {
				delete(m, k)
			}
		}
	})
	return nil
}

func (ns *NS) Unbind(src fs.FS, srcPath, dstPath string) error {
	if !fs.ValidPath(srcPath) {
		return &fs.PathError{Op: "unbind", Path: srcPath, Err: fs.ErrNotExist}
	}
	if !fs.ValidPath(dstPath) {
		return &fs.PathError{Op: "unbind", Path: dstPath, Err: fs.ErrNotExist}
	}

	// Resolve the source path first, just like in Bind. This may call back
	// into the namespace, so it must run outside the write lock.
	rfsys, rname, err := fs.Resolve(src, fs.ContextFor(ns), srcPath)
	if err != nil {
		return err
	}

	ns.mutate(func(m map[string][]bindTarget) {
		m[dstPath] = slices.DeleteFunc(m[dstPath], func(ref bindTarget) bool {
			return fs.Equal(ref.fs, rfsys) && ref.path == rname
		})
		if len(m[dstPath]) == 0 {
			delete(m, dstPath)
		}
	})

	return nil
}

// Bind adds a file or directory to the namespace.
// If specified, mode controls the order of the bindings.
// Only the first mode is used. If not specified, ModeAfter is used.
func (ns *NS) Bind(src fs.FS, srcPath, dstPath string, mode ...BindMode) error {
	if src == nil {
		return &fs.PathError{Op: "bind", Path: srcPath, Err: fs.ErrInvalid}
	}
	if !fs.ValidPath(srcPath) {
		return &fs.PathError{Op: "bind", Path: srcPath, Err: fs.ErrNotExist}
	}
	if !fs.ValidPath(dstPath) {
		return &fs.PathError{Op: "bind", Path: dstPath, Err: fs.ErrNotExist}
	}

	// Resolve, allocate, open, and stat the source outside any lock --
	// these can call into other filesystems (and recurse back through this
	// NS). Only the final mutation of the bindings map is serialized.
	rfsys, rname, err := fs.Resolve(src, fs.ContextFor(ns), srcPath)
	if err != nil {
		return err
	}

	// If the source filesystem implements BindAllocator,
	// use it to allocate a new filesystem for the binding.
	if allocator, ok := rfsys.(BindAllocator); ok {
		rfsys, err = allocator.BindAllocFS(srcPath)
		if err != nil {
			return err
		}
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

	var m BindMode
	if len(mode) == 0 {
		m = ModeAfter
	} else {
		m = mode[0]
	}
	if m != ModeAfter && m != ModeBefore && m != ModeReplace {
		return &fs.PathError{Op: "bind", Path: dstPath, Err: fs.ErrInvalid}
	}

	ns.mutate(func(b map[string][]bindTarget) {
		switch m {
		case ModeAfter:
			b[dstPath] = append([]bindTarget{ref}, b[dstPath]...)
		case ModeBefore:
			b[dstPath] = append(b[dstPath], ref)
		case ModeReplace:
			b[dstPath] = []bindTarget{ref}
		}
	})
	return nil
}

// Binds returns all fileinfo for bindings in a directory
func (ns *NS) Binds(name string) ([]fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "binds", Path: name, Err: fs.ErrNotExist}
	}
	var result []fs.FileInfo
	for path, refs := range ns.snapshot() {
		if strings.HasPrefix(path, name+"/") {
			fname := strings.Split(strings.TrimPrefix(path, name+"/"), "/")[0]
			for _, ref := range refs {
				fi, err := ref.fileInfo(context.Background(), fname)
				if err != nil {
					continue
				}
				result = append(result, fi)
			}
		}
	}
	return result, nil
}

func (ns *NS) String() string {
	var lines []string
	for dst, b := range ns.snapshot() {
		for _, ref := range b {
			lines = append(lines, fmt.Sprintf("%s -> %s:%s", dst, reflect.TypeOf(ref.fs), ref.path))
		}
	}
	return strings.Join(lines, "\n")
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

	// we implement Stat to try and avoid using Open for Stat
	// since it involves calling Stat on all sub filesystem roots
	// which could lead to stack overflow when there is a cycle.

	if name == "." {
		return fskit.Entry(name, fs.ModeDir|0755), nil
	}

	// Check direct bindings since they don't get resolved by the resolver.
	// todo: again, if there is a direct binding by this name, it might also
	// exist as a subpath of another binding. so this is not correct.
	if refs, exists := ns.snapshot()[name]; exists {
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
func (ns *NS) Open(name string) (fs.File, error) {
	ctx := fs.WithOrigin(ns.ctx, ns, name, "open")
	return ns.OpenContext(ctx, name)
}

// OpenContext ...
func (ns *NS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	ctx = fs.WithOrigin(ctx, ns, name, "open")

	b := ns.snapshot()

	var dir *fskit.Node
	var dirEntries []fs.DirEntry
	var foundDir bool

	// Check direct bindings
	if refs, exists := b[name]; exists {
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
	for p := range b {
		bindPaths = append(bindPaths, p)
	}
	for _, bindPath := range fskit.MatchPaths(bindPaths, name) {
		for _, ref := range b[bindPath] {
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
		for fname, refs := range b {
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
		for fname, refs := range b {
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
