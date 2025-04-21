package fs

import (
	"context"
	"slices"
)

var (
	// NoFollowContextKey is the context key for the no-follow flag.
	NoFollowContextKey = &contextKey{"no-follow"}
	// OriginContextKey is the context key for the originating filesystem.
	OriginContextKey = &contextKey{"origin"}
	// FilepathContextKey is the context key for the fully qualified path relative to the origin.
	FilepathContextKey = &contextKey{"filepath"}
	// ReadOnlyContextKey is the context key for the read-only flag.
	ReadOnlyContextKey = &contextKey{"read-only"}

	OpContextKey = &contextKey{"op"}
)

type contextFS interface {
	Context() context.Context
}

func ContextFor(fsys FS) context.Context {
	if cfs, ok := fsys.(contextFS); ok {
		return cfs.Context()
	}
	return context.Background()
}

// FollowSymlinks returns true if symlinks should be followed and is intended
// to be used on contexts passed to OpenContext, et al.
func FollowSymlinks(ctx context.Context) bool {
	if ctx == nil {
		return true
	}
	return ctx.Value(NoFollowContextKey) == nil
}

// IsReadOnly returns true if the context is read-only.
func IsReadOnly(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	return ctx.Value(ReadOnlyContextKey) != nil
}

// WithReadOnly returns a new context with the ReadOnlyContextKey set to true.
func WithReadOnly(ctx context.Context) context.Context {
	if ctx == nil {
		return nil
	}
	if IsReadOnly(ctx) {
		return ctx
	}
	return context.WithValue(ctx, ReadOnlyContextKey, true)
}

// WithNoFollow returns a new context with the NoFollowContextKey set to true.
func WithNoFollow(ctx context.Context) context.Context {
	if ctx == nil {
		return nil
	}
	return context.WithValue(ctx, NoFollowContextKey, true)
}

// Origin returns the origin filesystem and filepath for the given context. The
// filepath may be empty but as long as there is an origin it will return true.
func Origin(ctx context.Context) (FS, string, bool) {
	if ctx == nil {
		return nil, "", false
	}
	fsys, ok := ctx.Value(OriginContextKey).(FS)
	if !ok {
		return nil, "", false
	}
	filepath, ok := ctx.Value(FilepathContextKey).(string)
	if ok {
		return fsys, filepath, true
	}
	return fsys, "", true
}

// Op returns the operation for the given context.
// TODO: this and other origin stuff should be a struct in a single context value.
func Op(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v := ctx.Value(OpContextKey)
	if v == nil {
		return ""
	}
	return v.(string)
}

// WithOrigin returns a new context with the OriginContextKey set to the given
// filesystem unless there is already an origin filesystem in the context.
func WithOrigin(ctx context.Context, fsys FS, name string, op string) context.Context {
	if ctx == nil {
		return nil
	}
	if _, _, ok := Origin(ctx); ok {
		return ctx
	}
	ctx = WithFilepath(ctx, name)
	ctx = WithOp(ctx, op)
	if slices.Contains([]string{"open", "stat", "readdir", "readlink"}, op) {
		ctx = WithReadOnly(ctx)
	}
	// log.Printf("%s %s [%T]\n", op, name, fsys)
	return context.WithValue(ctx, OriginContextKey, fsys)
}

// WithFilepath returns a new context with the FilepathContextKey set to the
// given filepath unless there is already a filepath in the context.
func WithFilepath(ctx context.Context, filepath string) context.Context {
	if ctx == nil {
		return nil
	}
	if _, ok := ctx.Value(FilepathContextKey).(string); ok {
		return ctx
	}
	return context.WithValue(ctx, FilepathContextKey, filepath)
}

func WithOp(ctx context.Context, op string) context.Context {
	if ctx == nil {
		return nil
	}
	if _, ok := ctx.Value(OpContextKey).(string); ok {
		return ctx
	}
	return context.WithValue(ctx, OpContextKey, op)
}

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation. We use this
// for the context keys that are common to all filesystems.
type contextKey struct {
	name string
}

func (k *contextKey) String() string { return "fs context value " + k.name }
