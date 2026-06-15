package fs_test

import (
	"context"
	"strings"
	"testing"

	"tractor.dev/wanix/fs"
)

// mountFS resolves one mount prefix and passes the remainder to inner.
type mountFS struct {
	mount string
	inner fs.FS
}

func (m *mountFS) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (m *mountFS) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	if name == m.mount {
		return m.inner, ".", nil
	}
	if rest, ok := strings.CutPrefix(name, m.mount+"/"); ok {
		return m.inner, rest, nil
	}
	return m, name, nil
}

// leafFS is a minimal writable filesystem used as a resolve target.
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

func TestResolve_fixedPoint(t *testing.T) {
	leaf := leafFS{}
	root := &mountFS{
		mount: "a",
		inner: &mountFS{mount: "b", inner: leaf},
	}

	gotFS, gotName, err := fs.Resolve(root, context.Background(), "a/b/file.txt")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if gotFS != leaf {
		t.Fatalf("Resolve fs: got %T want leafFS", gotFS)
	}
	if gotName != "file.txt" {
		t.Fatalf("Resolve name: got %q want %q", gotName, "file.txt")
	}
}

func TestResolve_maxDepth(t *testing.T) {
	root := &cycleMount{mount: "a"}
	root.inner = root

	_, _, err := fs.Resolve(root, context.Background(), "a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/file")
	if err == nil {
		t.Fatal("expected error for resolve cycle exceeding max depth")
	}
	if !strings.Contains(err.Error(), "max depth") {
		t.Fatalf("expected max depth error, got: %v", err)
	}
}

// cycleMount always resolves to itself with the remainder of the path.
type cycleMount struct {
	mount string
	inner fs.FS
}

func (m *cycleMount) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (m *cycleMount) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	if rest, ok := strings.CutPrefix(name, m.mount+"/"); ok {
		return m.inner, rest, nil
	}
	return m, name, nil
}
