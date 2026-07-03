package allocfs

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"sync"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/bind"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/misc"
)

const maxSymlinkDepth = 8

// Allocator creates a resource for the given id.
type Allocator func(ctx context.Context, id string, opts map[string]string) (fs.FS, error)

// FS is a generic resource filesystem with /new and numbered resource paths.
type FS struct {
	mu        sync.RWMutex
	resources map[string]fs.FS
	aliases   map[string]string // name -> resource id
	nextID    int
	allocator Allocator
}

var (
	_ fs.SymlinkFS     = (*FS)(nil)
	_ fs.ReadlinkFS    = (*FS)(nil)
	_ fs.RouteFS       = (*FS)(nil)
	_ fs.OpenContextFS = (*FS)(nil)
	_ fs.StatContextFS = (*FS)(nil)
)

// New returns a resource filesystem. Reading /new allocates the next resource.
func New(allocator Allocator) *FS {
	return &FS{
		resources: make(map[string]fs.FS),
		aliases:   make(map[string]string),
		allocator: allocator,
	}
}

// Lookup returns the filesystem for a resource id.
func (f *FS) Lookup(id string) (fs.FS, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	r, ok := f.resources[id]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return r, nil
}

// Alloc creates a resource and stores it at the next id.
func (f *FS) Alloc(ctx context.Context, opts map[string]string) (fs.FS, string, error) {
	f.mu.Lock()
	f.nextID++
	id := strconv.Itoa(f.nextID)
	f.mu.Unlock()

	r, err := f.allocator(ctx, id, opts)
	if err != nil {
		return nil, "", err
	}

	f.mu.Lock()
	f.resources[id] = r
	f.mu.Unlock()
	return r, id, nil
}

// Symlink creates a name that refers to an existing resource id.
func (f *FS) Symlink(oldname, newname string) error {
	oldname = path.Clean(oldname)
	newname = path.Clean(newname)
	if !fs.ValidPath(newname) || newname == "." || strings.Contains(newname, "/") {
		return &fs.PathError{Op: "symlink", Path: newname, Err: fs.ErrNotExist}
	}
	if newname == "new" {
		return &fs.PathError{Op: "symlink", Path: newname, Err: fs.ErrExist}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.resources[oldname]; !ok {
		return &fs.PathError{Op: "symlink", Path: oldname, Err: fs.ErrNotExist}
	}
	if _, ok := f.resources[newname]; ok {
		return &fs.PathError{Op: "symlink", Path: newname, Err: fs.ErrExist}
	}
	if _, ok := f.aliases[newname]; ok {
		return &fs.PathError{Op: "symlink", Path: newname, Err: fs.ErrExist}
	}
	f.aliases[newname] = oldname
	return nil
}

// Readlink returns the resource id a symlink name refers to.
func (f *FS) Readlink(name string) (string, error) {
	name = path.Clean(name)
	f.mu.RLock()
	target, ok := f.aliases[name]
	f.mu.RUnlock()
	if !ok {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrNotExist}
	}
	return target, nil
}

func (f *FS) Open(name string) (fs.File, error) {
	return f.OpenContext(context.Background(), name)
}

func (f *FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if fs.FollowSymlinks(ctx) {
		resolved, err := f.resolve(name)
		if err != nil {
			return nil, err
		}
		name = resolved
	}
	return fs.OpenContext(ctx, f.rootFS(), name)
}

func (f *FS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	if fs.FollowSymlinks(ctx) {
		resolved, err := f.resolve(name)
		if err != nil {
			return nil, err
		}
		name = resolved
	}
	return fs.StatContext(ctx, f.rootFS(), name)
}

func (f *FS) Route(ctx context.Context, name string) (fs.FS, string, error) {
	if fs.FollowSymlinks(ctx) {
		resolved, err := f.resolve(name)
		if err != nil {
			return nil, "", err
		}
		name = resolved
	}
	return f.rootFS().Route(ctx, name)
}

func (f *FS) resolve(name string) (string, error) {
	if name == "." {
		return ".", nil
	}
	clean := path.Clean(name)
	seen := make(map[string]struct{})
	for range maxSymlinkDepth {
		head, tail, _ := strings.Cut(clean, "/")
		if head == "" || head == "." {
			return clean, nil
		}
		f.mu.RLock()
		target, ok := f.aliases[head]
		f.mu.RUnlock()
		if !ok {
			return clean, nil
		}
		if _, dup := seen[head]; dup {
			return "", &fs.PathError{Op: "resolve", Path: name, Err: fs.ErrInvalid}
		}
		seen[head] = struct{}{}
		clean = path.Join(target, tail)
	}
	return "", &fs.PathError{Op: "resolve", Path: name, Err: fs.ErrInvalid}
}

func (f *FS) rootFS() fskit.UnionFS {
	f.mu.RLock()
	defer f.mu.RUnlock()

	resources := fskit.MapFS(f.resources)
	aliases := make(fskit.MapFS, len(f.aliases))
	for name, target := range f.aliases {
		aliases[name] = fskit.RawNode([]byte(target), fs.ModeSymlink|0777)
	}

	return fskit.UnionFS{
		fskit.MapFS{
			"new": &newFS{OpenFunc: fskit.OpenFunc(f.openNew), f: f},
		},
		aliases,
		resources,
	}
}

type newFS struct {
	fskit.OpenFunc
	f *FS
}

var (
	_ bind.Allocator = (*newFS)(nil)
)

func (f *newFS) BindAlloc(ctx context.Context, src, dst string, opts map[string]string) (fs.FS, string, error) {
	fmt.Println("bind alloc", src, dst, opts)
	return f.f.Alloc(ctx, opts)
}

func (f *FS) openNew(ctx context.Context, name string) (fs.File, error) {
	if name != "." {
		return nil, fs.ErrNotExist
	}
	return misc.NewInvokeFile(func(opts map[string]string) (string, error) {
		_, id, err := f.Alloc(ctx, opts)
		if err != nil {
			return "", err
		}
		if _, fullpath, ok := fs.Origin(ctx); ok {
			return path.Join(path.Dir(fullpath), id), nil
		}
		return id, nil
	}), nil
}
