package fskit

import (
	"context"
	"errors"
	"log"

	"tractor.dev/wanix/fs"
)

// read-only union of filesystems
type UnionFS []fs.FS

var _ fs.RouteFS = UnionFS(nil)

func (f UnionFS) Open(name string) (fs.File, error) {
	ctx := fs.WithOrigin(context.Background(), f, name, "open")
	return f.OpenContext(ctx, name)
}

func (f UnionFS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	loc, err := fs.Walk(ctx, f, name)
	if err != nil {
		return nil, err
	}
	if !fs.Equal(loc.FS, f) {
		return fs.OpenContext(ctx, loc.FS, loc.Rel)
	}

	if name != "." {
		var entries []fs.DirEntry
		for _, fsys := range f {
			e, err := fs.ReadDirContext(ctx, fsys, name)
			if err != nil {
				continue
			}
			entries = append(entries, e...)
		}
		if len(entries) == 0 {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
		return DirFile(Entry(name, 0555), entries...), nil
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

func (f UnionFS) Route(ctx context.Context, name string) (fs.FS, string, error) {
	if len(f) == 0 {
		return nil, "", &fs.PathError{Op: "route", Path: name, Err: fs.ErrNotExist}
	}
	if len(f) == 1 {
		return f[0], name, nil
	}
	if name == "." && fs.IsReadOnly(ctx) {
		return f, name, nil
	}

	var toStat []fs.FS
	for _, fsys := range f {
		if router, ok := fsys.(fs.RouteFS); ok {
			next, rest, err := router.Route(ctx, name)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					// certainly does not have name
					continue
				}
				return nil, "", err
			}
			if !fs.IsReadOnly(ctx) {
				if _, ok := next.(fs.CreateFS); ok {
					return next, rest, nil
				}
			}
			if rest != name || !fs.Equal(next, fsys) {
				// certainly does have name
				return next, rest, nil
			}
		}
		toStat = append(toStat, fsys)
	}

	for _, fsys := range toStat {
		fi, err := fs.StatContext(ctx, fsys, name)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			continue // union may merge this directory across members
		}
		return fsys, name, nil
	}

	return f, name, nil
}
