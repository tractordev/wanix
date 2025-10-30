package memfs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

type FS struct {
	nodes map[string]*fskit.Node
	mu    sync.Mutex
	log   *slog.Logger
}

func New() *FS {
	fsys := &FS{nodes: make(map[string]*fskit.Node), log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	// Always ensure "." exists as the root directory
	fsys.nodes["."] = fskit.RawNode(".", fs.ModeDir|0755)
	fskit.SetSize(fsys.nodes["."], 2) // "." and ".."
	return fsys
}

func From(m fskit.MapFS) *FS {
	fsys := New()
	// First pass: create all nodes and implicit directories
	for name, node := range m {
		fsys.nodes[name] = fskit.RawNode(node)
		// Create implicit parent directories
		dir := path.Dir(name)
		for dir != "." {
			if _, exists := fsys.nodes[dir]; !exists {
				fsys.nodes[dir] = fskit.RawNode(dir, fs.ModeDir|0755)
			}
			dir = path.Dir(dir)
		}
	}
	// Second pass: set directory sizes based on children
	for name, node := range fsys.nodes {
		if node.IsDir() {
			// Count direct children
			count := 0
			if name == "." {
				// For root directory, count top-level entries
				for p := range fsys.nodes {
					if p != "." && !strings.Contains(p, "/") {
						count++
					}
				}
			} else {
				prefix := name + "/"
				for p := range fsys.nodes {
					if strings.HasPrefix(p, prefix) {
						rest := p[len(prefix):]
						if !strings.Contains(rest, "/") {
							count++
						}
					}
				}
			}
			// Set size to include "." and ".." plus actual entries
			fskit.SetSize(node, int64(2+count))
		}
	}
	return fsys
}

func (fsys *FS) SetLogger(logger *slog.Logger) {
	fsys.log = logger
}

func (fsys *FS) Clear() {
	fsys.mu.Lock()
	defer fsys.mu.Unlock()
	fsys.nodes = make(map[string]*fskit.Node)
	// Always ensure "." exists as the root directory
	fsys.nodes["."] = fskit.RawNode(".", fs.ModeDir|0755)
	fskit.SetSize(fsys.nodes["."], 2) // "." and ".."
}

func (fsys *FS) SetNode(name string, node *fskit.Node) {
	name = path.Clean(name)
	fsys.mu.Lock()
	defer fsys.mu.Unlock()
	fsys.nodes[name] = node
}

func (fsys *FS) Node(name string) (*fskit.Node, bool) {
	name = path.Clean(name)
	fsys.mu.Lock()
	defer fsys.mu.Unlock()
	node, ok := fsys.nodes[name]
	return node, ok
}

// updateDirSize updates the size of a directory based on its children.
// Must be called with fsys.mu held.
func (fsys *FS) updateDirSize(dir string) {
	node, ok := fsys.nodes[dir]
	if !ok {
		return
	}

	// Release lock temporarily to check if it's a directory (to avoid deadlock)
	fsys.mu.Unlock()
	isDir := node.IsDir()
	fsys.mu.Lock()

	if !isDir {
		return
	}

	count := 0
	if dir == "." {
		// For root directory, count top-level entries
		for p := range fsys.nodes {
			if p != "." && !strings.Contains(p, "/") {
				count++
			}
		}
	} else {
		prefix := dir + "/"
		for p := range fsys.nodes {
			if strings.HasPrefix(p, prefix) {
				rest := p[len(prefix):]
				if !strings.Contains(rest, "/") {
					count++
				}
			}
		}
	}

	// Release lock before calling SetSize to avoid deadlock
	fsys.mu.Unlock()
	fskit.SetSize(node, int64(2+count))
	fsys.mu.Lock()
}

func (fsys *FS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	return fsys.StatContext(context.Background(), name)
}

func (fsys *FS) StatContext(ctx context.Context, name string) (fi fs.FileInfo, err error) {
	defer func() {
		fsys.log.Debug("stat", "name", name, "err", err)
	}()
	f, err := fsys.OpenContext(fs.WithNoFollow(ctx), name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Clean name after opening, for use in symlink resolution
	name = path.Clean(name)
	fi, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if fs.FollowSymlinks(ctx) && fs.IsSymlink(fi.Mode()) {
		target, err := fs.Readlink(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("memfs: readlink %s: %w", name, err)
		}
		if origin, fullname, ok := fs.Origin(ctx); ok {
			if strings.HasPrefix(target, "/") {
				target = target[1:]
			} else {
				target = path.Join(strings.TrimSuffix(fullname, name), target)
			}
			return fs.StatContext(ctx, origin, target)
		} else {
			if strings.HasPrefix(target, "/") {
				fsys.log.Debug("statcontext", "error", "no origin for absolute symlink", "name", name)
				return nil, fs.ErrInvalid
			} else {
				target = path.Join(path.Dir(name), target)
				return fs.StatContext(ctx, fsys, target)
			}
		}
	}
	return fi, nil
}

func (fsys *FS) OpenContext(ctx context.Context, name string) (f fs.File, err error) {
	defer func() {
		fsys.log.Debug("open", "name", name, "err", err)
	}()
	// Check validity before cleaning - paths like "./." and "file/." are invalid per fs.ValidPath
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	name = path.Clean(name)

	fsys.mu.Lock()
	n := fsys.nodes[name]
	fsys.mu.Unlock()

	if n != nil {
		// SetName and SetLogger called after releasing lock to avoid deadlock
		fskit.SetName(n, name)
		fskit.SetLogger(n, fsys.log)
	}
	if n != nil {
		if fs.FollowSymlinks(ctx) && fs.IsSymlink(n.Mode()) {
			target, err := fs.Readlink(fsys, name)
			if err != nil {
				return nil, fmt.Errorf("memfs: readlink %s: %w", name, err)
			}
			if origin, fullname, ok := fs.Origin(ctx); ok {
				if strings.HasPrefix(target, "/") {
					target = target[1:]
				} else {
					target = path.Join(strings.TrimSuffix(fullname, name), target)
				}
				return fs.OpenContext(ctx, origin, target)
			} else {
				if strings.HasPrefix(target, "/") {
					fsys.log.Debug("opencontext", "error", "no origin for absolute symlink", "name", name)
					return nil, fs.ErrInvalid
				} else {
					target = path.Join(path.Dir(name), target)
					return fs.OpenContext(ctx, fsys, target)
				}
			}
		}
		if !n.IsDir() {
			// Ordinary file
			return fs.OpenContext(ctx, n, ".")
		}
	}

	// Directory, possibly synthesized.
	// Note that file can be nil here: the map need not contain explicit parent directories for all its files.
	// But file can also be non-nil, in case the user wants to set metadata for the directory explicitly.
	// Either way, we need to construct the list of children of this directory.
	var list []*fskit.Node
	var need = make(map[string]bool)
	if name == "." {
		fsys.mu.Lock()
		for fname, fi := range fsys.nodes {
			i := strings.Index(fname, "/")
			if i < 0 {
				if fname != "." {
					list = append(list, fskit.RawNode(fi, fname))
				}
			} else {
				need[fname[:i]] = true
			}
		}
		fsys.mu.Unlock()
	} else {
		prefix := name + "/"
		fsys.mu.Lock()
		for fname, fi := range fsys.nodes {
			if strings.HasPrefix(fname, prefix) {
				felem := fname[len(prefix):]
				i := strings.Index(felem, "/")
				if i < 0 {
					list = append(list, fskit.RawNode(fi, felem))
				} else {
					need[fname[len(prefix):len(prefix)+i]] = true
				}
			}
		}
		fsys.mu.Unlock()
		// If the directory name is not in the map,
		// and there are no children of the name in the map,
		// then the directory is treated as not existing.
		if n == nil && list == nil && len(need) == 0 {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
	}
	for _, fi := range list {
		delete(need, fi.Name())
	}
	for name := range need {
		list = append(list, fskit.RawNode(name, fs.FileMode(fs.ModeDir|0755)))
	}
	slices.SortFunc(list, func(a, b *fskit.Node) int {
		return strings.Compare(a.Name(), b.Name())
	})

	if n == nil {
		n = fskit.RawNode(name, fs.ModeDir|0755)
	}
	var entries []fs.DirEntry
	for _, n := range list {
		entries = append(entries, n)
	}
	// Set directory size to include "." and ".." plus actual entries
	fskit.SetSize(n, int64(2+len(entries)))
	fskit.SetLogger(n, fsys.log)
	return fskit.DirFile(n, entries...), nil
}

func (fsys *FS) Create(name string) (f fs.File, err error) {
	defer func() {
		fsys.log.Debug("create", "name", name, "err", err)
	}()
	name = path.Clean(name)
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrNotExist}
	}

	// Check if parent directory exists
	dir := path.Dir(name)
	if dir != "." {
		ok, err := fs.Exists(fsys, dir)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrNotExist}
		}
	}

	fsys.mu.Lock()
	node := fskit.Entry(name, fs.FileMode(0644), time.Now())
	fskit.SetLogger(node, fsys.log)
	fsys.nodes[name] = node
	// Update parent directory size
	fsys.updateDirSize(dir)
	fsys.mu.Unlock()

	// Open the file AFTER releasing fsys.mu to avoid deadlock
	return node.Open(".")
}

func (fsys *FS) Mkdir(name string, perm fs.FileMode) (err error) {
	defer func() {
		fsys.log.Debug("mkdir", "name", name, "perm", perm, "err", err)
	}()
	name = path.Clean(name)
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if ok {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrExist}
	}

	dir := path.Dir(name)
	ok, err = fs.Exists(fsys, dir)
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
	}

	fsys.mu.Lock()
	defer fsys.mu.Unlock()
	node := fskit.Entry(name, perm|fs.ModeDir, time.Now())
	fskit.SetSize(node, 2) // Set initial size to 2 for "." and ".." entries
	fskit.SetLogger(node, fsys.log)
	fsys.nodes[name] = node
	// Update parent directory size
	fsys.updateDirSize(dir)
	return nil
}

func (fsys *FS) Chmod(name string, mode fs.FileMode) (err error) {
	defer func() {
		fsys.log.Debug("chmod", "name", name, "mode", mode, "err", err)
	}()
	name = path.Clean(name)
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chmod", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "chmod", Path: name, Err: fs.ErrNotExist}
	}

	fsys.mu.Lock()
	node := fsys.nodes[name]
	fsys.mu.Unlock()

	if node == nil {
		return &fs.PathError{Op: "chmod", Path: name, Err: fs.ErrNotExist}
	}

	// Get current mode and set new mode without holding fsys.mu to avoid deadlock
	currentMode := node.Mode()
	fskit.SetMode(node, currentMode&fs.ModeType|mode&0777)
	return nil
}

func (fsys *FS) Chown(name string, uid, gid int) (err error) {
	defer func() {
		fsys.log.Debug("chown", "name", name, "uid", uid, "gid", gid, "err", err)
	}()
	name = path.Clean(name)
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chown", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "chown", Path: name, Err: fs.ErrNotExist}
	}

	fsys.mu.Lock()
	node := fsys.nodes[name]
	fsys.mu.Unlock()

	if node == nil {
		return &fs.PathError{Op: "chown", Path: name, Err: fs.ErrNotExist}
	}

	// Set uid/gid without holding fsys.mu to avoid deadlock
	fskit.SetUid(node, uid)
	fskit.SetGid(node, gid)
	return nil
}

func (fsys *FS) Chtimes(name string, atime, mtime time.Time) (err error) {
	defer func() {
		fsys.log.Debug("chtimes", "name", name, "atime", atime, "mtime", mtime, "err", err)
	}()
	name = path.Clean(name)
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chtimes", Path: name, Err: fs.ErrNotExist}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "chtimes", Path: name, Err: fs.ErrNotExist}
	}

	fsys.mu.Lock()
	node := fsys.nodes[name]
	fsys.mu.Unlock()

	if node == nil {
		return &fs.PathError{Op: "chtimes", Path: name, Err: fs.ErrNotExist}
	}

	// Set modTime without holding fsys.mu to avoid deadlock
	fskit.SetModTime(node, mtime)
	return nil
}

func (fsys *FS) Truncate(name string, size int64) (err error) {
	defer func() {
		fsys.log.Debug("truncate", "name", name, "size", size, "err", err)
	}()
	name = path.Clean(name)
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "truncate", Path: name, Err: fs.ErrNotExist}
	}

	fsys.mu.Lock()
	node := fsys.nodes[name]
	fsys.mu.Unlock()

	if node == nil {
		return &fs.PathError{Op: "truncate", Path: name, Err: fs.ErrNotExist}
	}

	// Get current data and resize it
	data := node.Data()

	if size < 0 {
		return &fs.PathError{Op: "truncate", Path: name, Err: fs.ErrInvalid}
	}

	if size == int64(len(data)) {
		// No change needed
		return nil
	}

	var newData []byte
	if size > int64(len(data)) {
		// Extend with null bytes
		newData = make([]byte, size)
		copy(newData, data)
	} else {
		// Truncate to size
		newData = data[:size]
	}

	// Update the existing node's data without replacing the node
	fskit.SetData(node, newData)
	return nil
}

func (fsys *FS) Remove(name string) (err error) {
	defer func() {
		fsys.log.Debug("remove", "name", name, "err", err)
	}()
	name = path.Clean(name)
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	// Prevent removing the root directory
	if name == "." {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
	}

	ok, err := fs.Exists(fsys, name)
	if err != nil {
		return err
	}
	if !ok {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	if isDir, err := fs.IsDir(fsys, name); err != nil {
		return err
	} else if isDir {
		empty, err := fs.IsEmpty(fsys, name)
		if err != nil {
			return err
		}
		if !empty {
			return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotEmpty}
		}
	}

	// TODO: RemoveAll, gets into synthesized directories

	dir := path.Dir(name)
	fsys.mu.Lock()
	defer fsys.mu.Unlock()
	delete(fsys.nodes, name)
	// Update parent directory size
	fsys.updateDirSize(dir)
	return nil
}

func (fsys *FS) Rename(oldpath, newpath string) (err error) {
	defer func() {
		fsys.log.Debug("rename", "oldpath", oldpath, "newpath", newpath, "err", err)
	}()
	oldpath = path.Clean(oldpath)
	newpath = path.Clean(newpath)
	if !fs.ValidPath(oldpath) || !fs.ValidPath(newpath) {
		return &fs.PathError{Op: "rename", Path: oldpath, Err: fs.ErrNotExist}
	}

	if oldpath == newpath {
		return nil
	}

	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	// Check if source exists
	oldNode, exists := fsys.nodes[oldpath]
	if !exists {
		return &fs.PathError{Op: "rename", Path: oldpath, Err: fs.ErrNotExist}
	}

	// Check if parent directory of newpath exists
	if newDir := path.Dir(newpath); newDir != "." {
		if parentNode, exists := fsys.nodes[newDir]; !exists || !parentNode.IsDir() {
			return &fs.PathError{Op: "rename", Path: newpath, Err: fs.ErrNotExist}
		}
	}

	// Handle destination if it exists (POSIX overwrite rules)
	if newNode, exists := fsys.nodes[newpath]; exists {
		// If destination is a directory, check if it's empty
		if newNode.IsDir() {
			// Check if directory is empty
			prefix := newpath + "/"
			for p := range fsys.nodes {
				if strings.HasPrefix(p, prefix) {
					return &fs.PathError{Op: "rename", Path: newpath, Err: fs.ErrExist}
				}
			}
			// Empty directory: remove it and all its descendants (should be none)
			delete(fsys.nodes, newpath)
		} else {
			// Destination is a file: remove it
			delete(fsys.nodes, newpath)
		}
	}

	// If oldpath is a directory, rewrite all descendant keys
	if oldNode.IsDir() {
		toRename := make(map[string]*fskit.Node)
		prefix := oldpath + "/"

		// Collect all nodes to rename
		for p, n := range fsys.nodes {
			if p == oldpath || strings.HasPrefix(p, prefix) {
				np := newpath + strings.TrimPrefix(p, oldpath)
				toRename[np] = n
			}
		}

		// Delete old paths
		for p := range toRename {
			oldp := oldpath + strings.TrimPrefix(p, newpath)
			delete(fsys.nodes, oldp)
		}

		// Add new paths
		for np, n := range toRename {
			fsys.nodes[np] = n
		}
	} else {
		// Simple file rename
		fsys.nodes[newpath] = oldNode
		delete(fsys.nodes, oldpath)
	}

	// Update parent directory sizes
	oldDir := path.Dir(oldpath)
	newDir := path.Dir(newpath)
	fsys.updateDirSize(oldDir)
	if oldDir != newDir {
		fsys.updateDirSize(newDir)
	}

	return nil
}

func (fsys *FS) Symlink(oldname, newname string) (err error) {
	defer func() {
		fsys.log.Debug("symlink", "oldname", oldname, "newname", newname, "err", err)
	}()
	newname = path.Clean(newname)
	if !fs.ValidPath(newname) {
		return &fs.PathError{Op: "symlink", Path: oldname, Err: fs.ErrInvalid}
	}

	// Check if parent directory exists
	dir := path.Dir(newname)
	if dir != "." {
		ok, err := fs.Exists(fsys, dir)
		if err != nil {
			return err
		}
		if !ok {
			return &fs.PathError{Op: "symlink", Path: newname, Err: fs.ErrNotExist}
		}
	}

	fsys.mu.Lock()
	defer fsys.mu.Unlock()
	// symlinks don't care if target exists so we can just create it
	fsys.nodes[newname] = fskit.RawNode([]byte(oldname), fs.FileMode(0777)|fs.ModeSymlink)
	// Update parent directory size
	fsys.updateDirSize(dir)
	return nil
}

func (fsys *FS) Readlink(name string) (link string, err error) {
	defer func() {
		fsys.log.Debug("readlink", "name", name, "err", err)
	}()
	name = path.Clean(name)
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrNotExist}
	}

	fsys.mu.Lock()
	defer fsys.mu.Unlock()

	n, ok := fsys.nodes[name]
	if !ok {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrNotExist}
	}

	if !fs.IsSymlink(n.Mode()) {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}

	return string(n.Data()), nil
}
