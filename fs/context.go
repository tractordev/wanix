package fs

import "context"

type OpenContextFS interface {
	FS
	OpenContext(ctx context.Context, name string) (File, error)
}

// OpenContext is a helper that opens a file with the given context and name
// falling back to Open if context is not supported.
func OpenContext(fsys FS, ctx context.Context, name string) (File, error) {
	if o, ok := fsys.(OpenContextFS); ok {
		return o.OpenContext(ctx, name)
	}
	return fsys.Open(name)
}
