package bind

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

// Entry represents a reference to a name in a specific filesystem.
type Entry struct {
	FS      fs.FS
	Path    string
	Info    fs.FileInfo
	Options map[string]string
}

// FileInfo returns file info for the binding with the given display name.
func (e *Entry) FileInfo(fname string) (*fskit.Node, error) {
	// Use the file info captured at bind time. Re-statting every bind target
	// during namespace directory synthesis can create recursive/stat storms
	// against remote filesystems (notably p9 guest mounts), which can starve
	// in-flight directory reads.
	return fskit.RawNode(e.Info, fname), nil
}

// Table is a copy-on-write bind map with Plan9-style bind semantics.
//
// Concurrency model: the bindings map lives behind an atomic.Pointer;
// reads grab a snapshot lock-free. Writers (Bind/Unbind/UnbindAll) serialize
// via writeMu, copy the current map, mutate the copy, and atomically swap it in.
type Table struct {
	bindings atomic.Pointer[map[string][]Entry]
	writeMu  sync.Mutex
}

// New returns an empty bind table.
func New() *Table {
	t := &Table{}
	m := make(map[string][]Entry)
	t.bindings.Store(&m)
	return t
}

// Snapshot returns the current bindings map. Callers MUST treat it as
// read-only -- the returned map and its slices may be shared with other
// readers and with prior snapshots.
func (t *Table) Snapshot() map[string][]Entry {
	return *t.bindings.Load()
}

func (t *Table) mutate(fn func(m map[string][]Entry)) {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	cur := *t.bindings.Load()
	cp := make(map[string][]Entry, len(cur))
	for k, v := range cur {
		cp[k] = slices.Clone(v)
	}
	fn(cp)
	t.bindings.Store(&cp)
}

// Clone returns a deep copy of the table.
func (t *Table) Clone() *Table {
	cur := t.Snapshot()
	b := make(map[string][]Entry, len(cur))
	for k, v := range cur {
		b[k] = slices.Clone(v)
	}
	out := &Table{}
	out.bindings.Store(&b)
	return out
}

// Route performs one routing step through the bind table. self is the
// filesystem that owns this table and is returned for union/multi-bind cases.
//
// For paths under a multi-bind point, routing returns the first binding
// that contains the name. Merged directory views are built by the owner's
// Open implementation (recursive union), not here.
func (t *Table) Route(ctx context.Context, self fs.FS, name string) (fs.FS, string, error) {
	b := t.Snapshot()

	// todo: if there is a direct binding by this name, it might also
	// exist as a subpath of another binding. so this is not correct.
	if refs, ok := b[name]; ok {
		if len(refs) == 1 {
			return refs[0].FS, refs[0].Path, nil
		}
		if !fs.IsReadOnly(ctx) {
			for _, ref := range refs {
				if _, ok := ref.FS.(fs.CreateFS); ok {
					return ref.FS, ref.Path, nil
				}
			}
		}
		return self, name, nil
	}

	var bindPaths []string
	for p := range b {
		bindPaths = append(bindPaths, p)
	}
	for _, bindPath := range fskit.MatchPaths(bindPaths, name) {
		refs := b[bindPath]
		relativeName := strings.Trim(strings.TrimPrefix(name, bindPath), "/")
		var toStat []Entry

		for _, ref := range refs {
			fullName := path.Join(ref.Path, relativeName)
			if router, ok := ref.FS.(fs.RouteFS); ok {
				next, rest, err := router.Route(ctx, fullName)
				if err != nil {
					if errors.Is(err, fs.ErrNotExist) {
						continue
					}
					return next, rest, err
				}
				if rest != fullName || !fs.Equal(next, ref.FS) {
					return next, rest, nil
				}
			}
			toStat = append(toStat, ref)
		}

		for _, ref := range toStat {
			fullName := path.Join(ref.Path, relativeName)
			_, err := fs.LstatContext(ctx, ref.FS, fullName)
			if err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					log.Println("resolve stat:", err)
				}
				continue
			}
			return ref.FS, fullName, nil
		}

		if slices.Contains([]string{"create", "mkdir", "symlink"}, fs.Op(ctx)) {
			for _, ref := range toStat {
				fullName := path.Join(ref.Path, relativeName)
				_, err := fs.StatContext(ctx, ref.FS, path.Dir(fullName))
				if err != nil {
					continue
				}
				return ref.FS, fullName, nil
			}
		}
	}

	return self, name, nil
}

// UnbindAll removes all bindings except those for which keep returns true.
func (t *Table) UnbindAll(keep func(dst string) bool) {
	t.mutate(func(m map[string][]Entry) {
		for k := range m {
			if !keep(k) {
				delete(m, k)
			}
		}
	})
}

// Unbind removes a binding matching the resolved source.
func (t *Table) Unbind(ctx context.Context, src fs.FS, srcPath, dstPath string) error {
	if !fs.ValidPath(srcPath) {
		return &fs.PathError{Op: "unbind", Path: srcPath, Err: fs.ErrNotExist}
	}
	if !fs.ValidPath(dstPath) {
		return &fs.PathError{Op: "unbind", Path: dstPath, Err: fs.ErrNotExist}
	}

	rfsys, rname, err := fs.Resolve(src, ctx, srcPath)
	if err != nil {
		return err
	}

	t.mutate(func(m map[string][]Entry) {
		m[dstPath] = slices.DeleteFunc(m[dstPath], func(ref Entry) bool {
			return fs.Equal(ref.FS, rfsys) && ref.Path == rname
		})
		if len(m[dstPath]) == 0 {
			delete(m, dstPath)
		}
	})

	return nil
}

// Bind adds a file or directory to the table.
// Placement defaults to fs.BindAfter. Only the first placement option is used.
func (t *Table) Bind(ctx context.Context, src fs.FS, srcPath, dstPath string, opts ...fs.BindOption) error {
	if src == nil {
		return &fs.PathError{Op: "bind", Path: srcPath, Err: fs.ErrInvalid}
	}
	if !fs.ValidPath(srcPath) {
		return &fs.PathError{Op: "bind", Path: srcPath, Err: fs.ErrNotExist}
	}
	if !fs.ValidPath(dstPath) {
		return &fs.PathError{Op: "bind", Path: dstPath, Err: fs.ErrNotExist}
	}

	rfsys, rname, err := fs.Resolve(src, ctx, srcPath)
	if err != nil {
		return err
	}

	// Support deprecated vfs.BindAllocator via structural typing.
	if allocator, ok := rfsys.(interface {
		BindAllocFS(name string) (fs.FS, error)
	}); ok {
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

	ref := Entry{
		FS:      rfsys,
		Path:    rname,
		Info:    fi,
		Options: fs.ParseBindOptions(opts...),
	}

	placement := fs.BindPlacement(opts...)

	t.mutate(func(b map[string][]Entry) {
		switch placement {
		case fs.BindAfter:
			b[dstPath] = append([]Entry{ref}, b[dstPath]...)
		case fs.BindBefore:
			b[dstPath] = append(b[dstPath], ref)
		case fs.BindReplace:
			b[dstPath] = []Entry{ref}
		}
	})
	return nil
}

// Binds returns file info for bindings in a directory.
func (t *Table) Binds(name string) ([]fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "binds", Path: name, Err: fs.ErrNotExist}
	}
	var result []fs.FileInfo
	for path, refs := range t.Snapshot() {
		if strings.HasPrefix(path, name+"/") {
			fname := strings.Split(strings.TrimPrefix(path, name+"/"), "/")[0]
			for _, ref := range refs {
				fi, err := ref.FileInfo(fname)
				if err != nil {
					continue
				}
				result = append(result, fi)
			}
		}
	}
	return result, nil
}

// String returns a debug representation of all bindings.
func (t *Table) String() string {
	var lines []string
	for dst, b := range t.Snapshot() {
		for _, ref := range b {
			lines = append(lines, fmt.Sprintf("%s -> %s:%s", dst, reflect.TypeOf(ref.FS), ref.Path))
		}
	}
	return strings.Join(lines, "\n")
}
