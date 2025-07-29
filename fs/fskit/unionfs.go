package fskit

import (
	"context"
	"errors"
	"log"

	"tractor.dev/wanix/fs"
)

// read-only union of filesystems
type UnionFS []fs.FS

func (f UnionFS) Open(name string) (fs.File, error) {
	ctx := fs.WithOrigin(context.Background(), f, name, "open")
	return f.OpenContext(ctx, name)
}

func (f UnionFS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	rfsys, rname, err := f.ResolveFS(ctx, name)
	if err != nil {
		return nil, err
	}
	if rname != name || !fs.Equal(rfsys, f) {
		return fs.OpenContext(ctx, rfsys, rname)
	}

	if name != "." {
		//log.Printf("non-root open: %s (=> %T %s)", name, rfsys, rname)
		// if non-root open and not resolved, it does not exist
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	var entries []fs.DirEntry
	for _, fsys := range f {
		e, err := fs.ReadDirContext(ctx, fsys, name)
		if err != nil {
			log.Printf("readdir: %v %T %s\n", err, fsys, name)
			continue
		}
		entries = append(entries, e...)
	}

	return DirFile(Entry(name, 0555), entries...), nil
}

func (f UnionFS) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	if len(f) == 0 {
		return nil, "", &fs.PathError{Op: "resolve", Path: name, Err: fs.ErrNotExist}
	}
	if len(f) == 1 {
		return f[0], name, nil
	}
	if name == "." && fs.IsReadOnly(ctx) {
		return f, name, nil
	}

	var toStat []fs.FS
	for _, fsys := range f {
		if resolver, ok := fsys.(fs.ResolveFS); ok {
			rfsys, rname, err := resolver.ResolveFS(ctx, name)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					// certainly does not have name
					continue
				}
				return rfsys, rname, err
			}
			if !fs.IsReadOnly(ctx) {
				if _, ok := rfsys.(fs.CreateFS); ok {
					return rfsys, rname, nil
				}
			}
			if rname != name || !fs.Equal(rfsys, fsys) {
				// certainly does have name
				return rfsys, rname, nil
			}
		}
		toStat = append(toStat, fsys)
	}

	for _, fsys := range toStat {
		_, err := fs.StatContext(ctx, fsys, name)
		if err != nil {
			continue
		}
		if fs.IsReadOnly(ctx) {
			return fsys, name, nil
		}
		if _, ok := fsys.(fs.CreateFS); ok {
			return fsys, name, nil
		}
	}

	return f, name, nil
}
