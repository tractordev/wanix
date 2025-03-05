package namespace

import "context"

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation.
type contextKey struct {
	name string
}

func (k *contextKey) String() string { return "namespace context value " + k.name }

var (
	NamespaceContextKey = &contextKey{"namespace"}
	PathContextKey      = &contextKey{"path"}
)

func FromContext(ctx context.Context) (*FS, string, bool) {
	ns, ok := ctx.Value(NamespaceContextKey).(*FS)
	if !ok {
		return nil, "", false
	}
	path, ok := ctx.Value(PathContextKey).(string)
	return ns, path, ok
}
