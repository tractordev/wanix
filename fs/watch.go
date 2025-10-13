package fs

import "context"

type WatchFS interface {
	FS
	Watch(ctx context.Context, name string, exclude ...string) (<-chan Event, error)
}

type Event struct {
	Path string
	Op   string
	Err  error
}

// - use context to cancel watch
// - if context is done, return a closed channel
// - name can end with /... for recursive watch
// - exclude can be a glob pattern
func Watch(fsys FS, ctx context.Context, name string, exclude ...string) (<-chan Event, error) {
	if w, ok := fsys.(WatchFS); ok {
		return w.Watch(ctx, name, exclude...)
	}

	rfsys, rname, err := ResolveTo[WatchFS](fsys, ctx, name)
	if err == nil {
		return rfsys.Watch(ctx, rname, exclude...)
	}
	return nil, opErr(fsys, name, "watch", err)
}
