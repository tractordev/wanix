package namespace

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"testing/fstest"

	"tractor.dev/wanix/fs"

	"tractor.dev/wanix/fs/fskit"
)

func TestNamespace(t *testing.T) {
	// Create a test filesystem
	testFS := fstest.MapFS{
		"file1.txt":       {Data: []byte("content1")},
		"dir/file2.txt":   {Data: []byte("content2")},
		"dir/subdir/file": {Data: []byte("content3")},
	}

	// Create and initialize namespace
	ns := New(context.Background())

	// Test file binding
	ns.Bind(testFS, "file1.txt", "bound-file.txt", "replace")

	content, err := fs.ReadFile(ns, "bound-file.txt")
	if err != nil {
		t.Fatalf("Failed to open bound file: %v", err)
	}
	if string(content) != "content1" {
		t.Errorf("Expected content1, got %s", string(content))
	}

	// Test directory binding
	ns.Bind(testFS, ".", "bound-dir", "replace")
	content, err = fs.ReadFile(ns, "bound-dir/dir/file2.txt")
	if err != nil {
		t.Fatalf("Failed to open file in bound directory: %v", err)
	}
	if string(content) != "content2" {
		t.Errorf("Expected content2, got %s", string(content))
	}

	// Test ReadDir
	entries, err := fs.ReadDir(ns, "bound-dir/dir")
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}
	if len(entries) != 2 { // file2.txt and subdir
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}

	// Test file not found
	_, err = ns.Open("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestSubFS(t *testing.T) {
	ns := New(context.Background())

	subFS := fskit.MapFS{
		"subfile":     fskit.RawNode([]byte("rootsub")),
		"subdir/file": fskit.RawNode([]byte("subdirfile")),
	}

	rootFS := fskit.MapFS{
		"rootfile": fskit.RawNode([]byte("rootfile")),
		"rootsub":  subFS,
	}

	bindsubFS := fskit.MapFS{
		"bindfile": fskit.RawNode([]byte("bindfile")),
		"bindsub":  subFS,
	}

	ns.Bind(rootFS, ".", ".", "")
	ns.Bind(bindsubFS, ".", "bind", "")

	subfs, err := ns.Sub(".")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(subfs, rootFS) {
		t.Fatal("Sub(.) is not rootFS")
	}

	subfs, err = ns.Sub("bind")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(subfs, bindsubFS) {
		t.Fatal("Sub(bind) is not bindsubFS")
	}

	// requires fskit.MapFS to have proper Sub() implementation
	subfs, err = ns.Sub("bind/bindsub")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(subfs, subFS) {
		t.Fatalf("Sub(bind/bindsub) is not subFS: %T", subfs)
	}

	subfs, err = ns.Sub("rootsub/subdir")
	if err != nil {
		t.Fatal(err)
	}
	subdirfs, ok := subfs.(*fs.SubdirFS)
	if !ok {
		t.Fatalf("Sub(rootsub/subdir) is not a SubdirFS: %T", subfs)
	}
	if !reflect.DeepEqual(subdirfs.Fsys, subFS) {
		t.Fatalf("Sub(rootsub/subdir) is not a SubdirFS of subFS: %s %T", subdirfs.Dir, subdirfs.Fsys)
	}
}

func TestFileBindOverRootBind(t *testing.T) {
	abfs := fskit.MapFS{
		"a": fskit.RawNode([]byte("content1")),
		"b": fskit.RawNode([]byte("content2")),
	}

	cfs := fskit.MapFS{
		"c": abfs,
	}

	ns := New(context.Background())
	if err := ns.Bind(abfs, ".", ".", ""); err != nil {
		t.Fatal(err)
	}
	if err := ns.Bind(cfs, "c", "c", ""); err != nil {
		t.Fatal(err)
	}

	e, err := fs.ReadDir(ns, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(e) != 3 {
		// a, b, c
		t.Fatalf("unexpected number of entries: %v", len(e))
	}
}

func TestRecursiveSubpathBind(t *testing.T) {
	ns := New(context.Background())

	loopFS := fskit.MapFS{
		"ctl": fskit.RawNode([]byte("content1")),
		"ns":  ns,
	}

	rootFS := fskit.MapFS{
		"inner": loopFS,
	}

	ns.Bind(rootFS, ".", ".", "")

	_, err := fs.StatContext(context.Background(), ns, ".")
	if err != nil {
		t.Fatal(err)
	}
}

func TestHiddenSelfBind(t *testing.T) {
	abFS := fskit.MapFS{
		"a": fskit.RawNode([]byte("content1")),
		"b": fskit.RawNode([]byte("content2")),
	}

	mfs := fskit.MapFS{
		"one":  abFS,
		"#two": abFS,
	}

	ns := New(context.Background())
	if err := ns.Bind(mfs, ".", ".", ""); err != nil {
		t.Fatal(err)
	}
	if err := ns.Bind(ns, "#two", "two", ""); err != nil {
		t.Fatal(err)
	}

	e, err := fs.ReadDir(ns, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(e) != 2 {
		// one, two
		t.Fatal("unexpected number of non-hidden entries")
	}

	e, err = fs.ReadDir(ns, "two")
	if err != nil {
		t.Fatal(err)
	}
	if len(e) != 2 {
		// a, b
		t.Fatal("unexpected number of non-hidden entries")
	}

	_, err = fs.Stat(ns, "two/a")
	if err != nil {
		t.Fatal(err)
	}
}

func TestNamespaceHidden(t *testing.T) {
	testFS := fstest.MapFS{
		"a":  {Data: []byte("content1")},
		"b":  {Data: []byte("content2")},
		"#c": {Data: []byte("hidden")},
	}

	ns := New(context.Background())
	ns.Bind(testFS, ".", "#foo", "replace")

	e, _ := fs.ReadDir(ns, ".")
	if len(e) != 0 {
		t.Fatal("expected empty root dir listing")
	}

	e, _ = fs.ReadDir(ns, "#foo")
	if len(e) != 2 {
		t.Fatal("expected only 2 files in #foo dir listing")
	}

	b, err := fs.ReadFile(ns, "#foo/#c")
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hidden" {
		t.Fatal("unexpected hidden file contents")
	}

}

func TestUnionBinding(t *testing.T) {
	// Create a test filesystem
	fs1 := fstest.MapFS{
		"file1.txt":       {Data: []byte("content1")},
		"dir/file2.txt":   {Data: []byte("content2")},
		"dir/subdir/file": {Data: []byte("content3")},
	}
	fs2 := fstest.MapFS{
		"fs2.txt": {Data: []byte("fs2")},
	}

	// Create namespace with union binding at root and in dir
	ns := New(context.Background())
	ns.Bind(fs1, ".", ".", "")
	ns.Bind(fs2, ".", ".", "")
	ns.Bind(fs2, ".", "dir", "")

	// Test ReadDir in root
	entries, err := fs.ReadDir(ns, ".")
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}
	if len(entries) != 3 { // file1.txt, fs2.txt, and dir
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}

	// Test ReadDir in dir
	entries, err = fs.ReadDir(ns, "dir")
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}
	if len(entries) != 3 { // file2.txt, fs2.txt, and subdir
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}
}

func TestBindingModes(t *testing.T) {
	// Create test filesystems
	fs1 := fstest.MapFS{"file.txt": {Data: []byte("fs1")}}
	fs2 := fstest.MapFS{"file.txt": {Data: []byte("fs2")}}

	// Test replace mode
	ns := New(context.Background())
	ns.Bind(fs1, ".", "test", "replace")
	ns.Bind(fs2, ".", "test", "replace")

	content, err := fs.ReadFile(ns, "test/file.txt")
	if err != nil {
		t.Fatalf("Failed to open bound file: %v", err)
	}
	if string(content) != "fs2" {
		t.Errorf("Expected fs2, got %s", string(content))
	}

	// Test after mode (default)
	ns = New(context.Background())
	ns.Bind(fs2, ".", "test", "")
	ns.Bind(fs1, ".", "test", "after")

	content, err = fs.ReadFile(ns, "test/file.txt")
	if err != nil {
		t.Fatalf("Failed to open bound file: %v", err)
	}
	if string(content) != "fs1" {
		t.Errorf("Expected fs1, got %s", string(content))
	}

	// Test before mode
	ns = New(context.Background())
	ns.Bind(fs1, ".", "test", "replace")
	ns.Bind(fs2, ".", "test", "before")

	content, err = fs.ReadFile(ns, "test/file.txt")
	if err != nil {
		t.Fatalf("Failed to open bound file: %v", err)
	}
	if string(content) != "fs1" {
		t.Errorf("Expected fs1, got %s", string(content))
	}

}

func TestSynthesizedDirectories(t *testing.T) {
	// Create test filesystem
	testFS := fstest.MapFS{
		"file.txt": {Data: []byte("content")},
	}

	// Bind a file in a deep path
	ns := New(context.Background())
	ns.Bind(testFS, "file.txt", "a/b/c/file.txt", "")

	// Test that we can read parent directories
	tests := []struct {
		path     string
		expected []string // expected entry names
	}{
		{".", []string{"a"}},
		{"a", []string{"b"}},
		{"a/b", []string{"c"}},
		{"a/b/c", []string{"file.txt"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			entries, err := fs.ReadDir(ns, tt.path)
			if err != nil {
				t.Fatalf("ReadDir(%q) error: %v", tt.path, err)
			}

			if len(entries) != len(tt.expected) {
				t.Errorf("ReadDir(%q) got %d entries, want %d", tt.path, len(entries), len(tt.expected))
			}

			// Check entry names
			var got []string
			for _, entry := range entries {
				got = append(got, entry.Name())
				// Verify directory status
				if tt.path != "a/b/c" && !entry.IsDir() {
					t.Errorf("Entry %q in %q should be a directory", entry.Name(), tt.path)
				}
			}

			// Sort both slices for comparison
			sort.Strings(got)
			sort.Strings(tt.expected)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ReadDir(%q) got entries %v, want %v", tt.path, got, tt.expected)
			}
		})
	}

	// Test that we can open synthesized directories
	for _, path := range []string{"a", "a/b", "a/b/c"} {
		t.Run("Open/"+path, func(t *testing.T) {
			f, err := ns.Open(path)
			if err != nil {
				t.Fatalf("Open(%q) error: %v", path, err)
			}
			defer f.Close()

			// Verify it's a directory
			info, err := f.Stat()
			if err != nil {
				t.Fatalf("Stat() error: %v", err)
			}
			if !info.IsDir() {
				t.Errorf("Expected %q to be a directory", path)
			}
		})
	}
	// Test directory binding with synthesized parents
	ns2 := New(context.Background())
	ns2.Bind(testFS, ".", "x/y/z", "")

	// Verify parent directories are synthesized
	dirs := []string{".", "x", "x/y", "x/y/z"}
	for _, dir := range dirs {
		t.Run("DirBind/"+dir, func(t *testing.T) {
			entries, err := fs.ReadDir(ns2, dir)
			if err != nil {
				t.Fatalf("ReadDir(%q) error: %v", dir, err)
			}

			if len(entries) != 1 {
				t.Fatalf("ReadDir(%q) got %d entries, want 1", dir, len(entries))
			}

			entry := entries[0]
			if dir != "x/y/z" && !entry.IsDir() {
				t.Errorf("Entry in %q should be a directory", dir)
			}

			var expectedName string
			switch dir {
			case ".":
				expectedName = "x"
			case "x":
				expectedName = "y"
			case "x/y":
				expectedName = "z"
			case "x/y/z":
				expectedName = "file.txt"
			}

			if entry.Name() != expectedName {
				t.Errorf("ReadDir(%q) got entry name %q, want %q", dir, entry.Name(), expectedName)
			}
		})
	}
}

func TestMkdirOnLeaf(t *testing.T) {
	ns := New(context.Background())

	memfs := fskit.MemFS{
		"file": fskit.RawNode([]byte("content")),
	}

	middlefs := fskit.MapFS{
		"dir": memfs,
	}

	ns.Bind(middlefs, ".", "sub", "")

	// first we'll use Sub manually to get the memfs

	subfs, err := ns.Sub("sub/dir")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(subfs, memfs) {
		t.Fatalf("Sub(sub/dir) is not memfs: %T", subfs)
	}

	err = fs.Mkdir(subfs, "newdir1", 0755)
	if err != nil {
		t.Fatal(err)
	}

	dir, err := fs.ReadDir(ns, "sub/dir")
	if err != nil {
		t.Fatal(err)
	}
	if len(dir) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(dir))
	}

	// now we'll use Mkdir on the namespace directly

	err = fs.Mkdir(ns, "sub/dir/newdir2", 0755)
	if err != nil {
		t.Fatal(err)
	}

	dir, err = fs.ReadDir(ns, "sub/dir")
	if err != nil {
		t.Fatal(err)
	}
	if len(dir) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(dir))
	}

	// now we'll use MkdirAll on the namespace directly

	err = fs.MkdirAll(ns, "sub/dir/newdir3/newdir4/newdir5", 0755)
	if err != nil {
		t.Fatal(err)
	}

	dir, err = fs.ReadDir(ns, "sub/dir/newdir3/newdir4")
	if err != nil {
		t.Fatal(err)
	}
	if len(dir) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(dir))
	}
}
