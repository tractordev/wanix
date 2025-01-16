package ns

import (
	"io/fs"
	"reflect"
	"sort"
	"testing"
	"testing/fstest"
)

func TestNamespace(t *testing.T) {
	// Create a test filesystem
	testFS := fstest.MapFS{
		"file1.txt":       {Data: []byte("content1")},
		"dir/file2.txt":   {Data: []byte("content2")},
		"dir/subdir/file": {Data: []byte("content3")},
	}

	// Create and initialize namespace
	ns := New()

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
	ns := New()
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
	ns := New()
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
	ns = New()
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
	ns = New()
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
	ns := New()
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
	ns2 := New()
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
