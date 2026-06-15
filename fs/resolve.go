package fs

import (
	"context"
	"fmt"
	"path"
)

type ResolveFS interface {
	FS
	ResolveFS(ctx context.Context, name string) (FS, string, error)
}

const maxResolveDepth = 32

// finishResolve follows ResolveFS until the filesystem and relative name
// reach a fixed point.
func finishResolve(ctx context.Context, fsys FS, name string) (FS, string, error) {
	for range maxResolveDepth {
		r, ok := fsys.(ResolveFS)
		if !ok {
			return fsys, name, nil
		}
		nfsys, nname, err := r.ResolveFS(ctx, name)
		if err != nil {
			return nil, "", err
		}
		if Equal(nfsys, fsys) && nname == name {
			return fsys, name, nil
		}
		fsys, name = nfsys, nname
	}
	return nil, "", fmt.Errorf("resolve: exceeded max depth")
}

// ResolveTo resolves the name to an FS extension type if possible. It uses
// ResolveFS if available, otherwise it falls back to SubFS.
func ResolveTo[T FS](fsys FS, ctx context.Context, name string) (T, string, error) {
	var tfsys T

	rfsys, rname, err := Resolve(fsys, ctx, name)
	if err != nil {
		return tfsys, "", err
	}

	if v, ok := rfsys.(T); ok {
		tfsys = v
	} else {
		return tfsys, "", fmt.Errorf("resolve: %w on %T", ErrNotSupported, rfsys)
	}

	return tfsys, rname, nil
}

// Resolve resolves to the FS directly containing the name returning that
// resolved FS and the relative name for that FS. It uses ResolveFS if
// available, otherwise it falls back to SubFS. If unable to resolve,
// it returns the original FS and the original name, but it can also
// return a PathError if .
func Resolve(fsys FS, ctx context.Context, name string) (rfsys FS, rname string, err error) {
	// defer func() {
	// 	if rname != name {
	// 		pc2, _, _, _ := runtime.Caller(2)
	// 		pc3, _, _, _ := runtime.Caller(3)
	// 		pc4, _, _, _ := runtime.Caller(4)
	// 		callers := []string{
	// 			path.Base(runtime.FuncForPC(pc2).Name()),
	// 			path.Base(runtime.FuncForPC(pc3).Name()),
	// 			path.Base(runtime.FuncForPC(pc4).Name()),
	// 		}
	// 		line := fmt.Sprintf("  [%T] %s => [%T] %s %v %v", fsys, name, rfsys, rname, err, callers)
	// 		log.Println(strings.ReplaceAll(line, "fskit.", ""))
	// 	}
	// }()
	if res, ok := fsys.(ResolveFS); ok {
		var resolved FS
		resolved, rname, err = res.ResolveFS(ctx, name)
		if err != nil {
			return
		}
		return finishResolve(ctx, resolved, rname)
	}

	if name == "." {
		rfsys = fsys
		rname = name
		return
	}

	dirfs, e := Sub(fsys, path.Dir(name))
	if e != nil {
		err = e
		return
	}

	if Equal(dirfs, fsys) {
		rfsys = fsys
		rname = name
		return finishResolve(ctx, rfsys, rname)
	}

	if subfs, ok := dirfs.(*SubdirFS); ok {
		rfsys = subfs.Fsys

		if Equal(subfs.Fsys, fsys) {
			rname = name
			return finishResolve(ctx, rfsys, rname)
		}

		rname, err = subfs.fullName("resolve", path.Base(name))
		if err != nil {
			return
		}
		return finishResolve(ctx, rfsys, rname)
	}

	rfsys = dirfs
	rname = path.Base(name)
	return finishResolve(ctx, rfsys, rname)
}
