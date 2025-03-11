package fs

import "context"

var (
	// NoFollowContextKey is the context key for the no-follow flag.
	NoFollowContextKey = &contextKey{"no-follow"}
	// OriginContextKey is the context key for the originating filesystem.
	OriginContextKey = &contextKey{"origin"}
	// FilepathContextKey is the context key for the fully qualified path relative to the origin.
	FilepathContextKey = &contextKey{"filepath"}
)

// FollowSymlinks returns true if symlinks should be followed and is intended
// to be used on contexts passed to OpenContext, et al.
func FollowSymlinks(ctx context.Context) bool {
	if ctx == nil {
		return true
	}
	return ctx.Value(NoFollowContextKey) == nil
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

// WithOrigin returns a new context with the OriginContextKey set to the given
// filesystem unless there is already an origin filesystem in the context.
func WithOrigin(ctx context.Context, fsys FS) context.Context {
	if ctx == nil {
		return nil
	}
	if _, _, ok := Origin(ctx); ok {
		return ctx
	}
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

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation. We use this
// for the context keys that are common to all filesystems.
type contextKey struct {
	name string
}

func (k *contextKey) String() string { return "fs context value " + k.name }
