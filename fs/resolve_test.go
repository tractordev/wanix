package fs_test

import (
	"context"
	"strings"
	"testing"

	"tractor.dev/wanix/fs"
)

// mountFS routes one mount prefix and passes the remainder to inner.
type mountFS struct {
	mount string
	inner fs.FS
}

func (m *mountFS) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (m *mountFS) Route(ctx context.Context, name string) (fs.FS, string, error) {
	if name == m.mount {
		return m.inner, ".", nil
	}
	if rest, ok := strings.CutPrefix(name, m.mount+"/"); ok {
		return m.inner, rest, nil
	}
	return m, name, nil
}

var _ fs.RouteFS = (*mountFS)(nil)

// leafFS is a minimal writable filesystem used as a route target.
type leafFS struct{}

func (leafFS) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (leafFS) Mkdir(name string, perm fs.FileMode) error { return nil }

var _ fs.MkdirFS = leafFS{}

func TestResolveTo_multiHop(t *testing.T) {
	leaf := leafFS{}
	root := &mountFS{
		mount: "a",
		inner: &mountFS{
			mount: "b",
			inner: &mountFS{
				mount: "c",
				inner: leaf,
			},
		},
	}

	got, rname, err := fs.ResolveTo[fs.MkdirFS](root, context.Background(), "a/b/c/dir")
	if err != nil {
		t.Fatalf("ResolveTo: %v", err)
	}
	if got != leaf {
		t.Fatalf("ResolveTo fs: got %T want leafFS", got)
	}
	if rname != "dir" {
		t.Fatalf("ResolveTo name: got %q want %q", rname, "dir")
	}
}

func TestWalk_fixedPoint(t *testing.T) {
	leaf := leafFS{}
	root := &mountFS{
		mount: "a",
		inner: &mountFS{mount: "b", inner: leaf},
	}

	loc, err := fs.Walk(context.Background(), root, "a/b/file.txt")
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if loc.FS != leaf {
		t.Fatalf("Walk fs: got %T want leafFS", loc.FS)
	}
	if loc.Rel != "file.txt" {
		t.Fatalf("Walk name: got %q want %q", loc.Rel, "file.txt")
	}
}

func TestWalk_maxDepth(t *testing.T) {
	root := &cycleMount{mount: "a"}
	root.inner = root

	_, err := fs.Walk(context.Background(), root, "a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/file")
	if err == nil {
		t.Fatal("expected error for route cycle or max depth")
	}
	if !strings.Contains(err.Error(), "max depth") && !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected max depth or cycle error, got: %v", err)
	}
}

// cycleMount always routes to itself with the remainder of the path.
type cycleMount struct {
	mount string
	inner fs.FS
}

func (m *cycleMount) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (m *cycleMount) Route(ctx context.Context, name string) (fs.FS, string, error) {
	if rest, ok := strings.CutPrefix(name, m.mount+"/"); ok {
		return m.inner, rest, nil
	}
	return m, name, nil
}

var _ fs.RouteFS = (*cycleMount)(nil)
