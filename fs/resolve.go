package fs

import (
	"context"
	"fmt"
	"reflect"
)

// RouteFS performs one path routing step from this filesystem.
//
// Given name relative to fsys, Route returns the filesystem that should handle
// the remainder and rest, the path relative to that filesystem. Route must not
// recurse into next; fs.Walk repeats Route until the path reaches a fixed point.
// Returning (fsys, name) unchanged means this filesystem owns the path.
type RouteFS interface {
	FS
	Route(ctx context.Context, name string) (next FS, rest string, err error)
}

const maxRouteDepth = 32

// Loc is a filesystem and a path relative to it.
type Loc struct {
	FS  FS
	Rel string
}

type routeKey struct {
	ptr uintptr
	rel string
}

func routeFSKey(fsys FS) uintptr {
	if fsys == nil {
		return 0
	}
	v := reflect.ValueOf(fsys)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return 0
	}
	return v.Pointer()
}

func routeProgress(before Loc, next FS, rest string) bool {
	return !Equal(before.FS, next) || before.Rel != rest
}

// Walk routes name through RouteFS layers until a fixed point.
func Walk(ctx context.Context, fsys FS, name string) (Loc, error) {
	loc := Loc{FS: fsys, Rel: name}
	seen := make(map[routeKey]struct{})

	for range maxRouteDepth {
		r, ok := loc.FS.(RouteFS)
		if !ok {
			return loc, nil
		}

		next, rest, err := r.Route(ctx, loc.Rel)
		if err != nil {
			return Loc{}, err
		}
		if next == nil {
			return Loc{}, fmt.Errorf("route: nil fs from %T", loc.FS)
		}
		if !routeProgress(loc, next, rest) {
			return loc, nil
		}

		key := routeKey{ptr: routeFSKey(next), rel: rest}
		if _, dup := seen[key]; dup {
			return Loc{}, fmt.Errorf("route: cycle at %T %q", next, rest)
		}
		seen[key] = struct{}{}

		loc = Loc{FS: next, Rel: rest}
	}

	return Loc{}, fmt.Errorf("route: exceeded max depth")
}

// ResolveTo resolves the name to an FS extension type if possible.
func ResolveTo[T FS](fsys FS, ctx context.Context, name string) (T, string, error) {
	var tfsys T

	loc, err := Walk(ctx, fsys, name)
	if err != nil {
		return tfsys, "", err
	}

	if v, ok := loc.FS.(T); ok {
		tfsys = v
	} else {
		return tfsys, "", fmt.Errorf("resolve: %w on %T", ErrNotSupported, loc.FS)
	}

	return tfsys, loc.Rel, nil
}

// Resolve routes name to the filesystem that directly contains it and the
// relative path on that filesystem.
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
	loc, err := Walk(ctx, fsys, name)
	if err != nil {
		return nil, "", err
	}
	return loc.FS, loc.Rel, nil
}
