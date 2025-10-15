package httpfs

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/memfs"
)

// Helper function to create a test server with a memfs backend
func newTestServer() (*memfs.FS, *httptest.Server, *FS) {
	memFS := memfs.New()
	server := httptest.NewServer(NewServer(memFS))
	client := New(server.URL, nil)
	return memFS, server, client
}

func TestBasicFileOperations(t *testing.T) {
	memFS, server, client := newTestServer()
	defer server.Close()

	ctx := context.Background()

	// Create a file
	file, err := client.CreateContext(ctx, "test.txt", []byte("Hello, World!"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	file.Close()

	// Verify file exists in memfs
	info, err := fs.Stat(memFS, "test.txt")
	if err != nil {
		t.Fatalf("File not found in memfs: %v", err)
	}
	if info.Size() != 13 {
		t.Errorf("Expected size 13, got %d", info.Size())
	}

	// Read the file through HTTP
	content, err := fs.ReadFile(client, "test.txt")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", string(content))
	}

	// Stat the file
	clientInfo, err := client.Stat("test.txt")
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if clientInfo.Size() != 13 {
		t.Errorf("Expected size 13, got %d", clientInfo.Size())
	}

	// Remove the file
	if err := client.Remove("test.txt"); err != nil {
		t.Fatalf("Failed to remove file: %v", err)
	}

	// Verify file is removed
	_, err = fs.Stat(memFS, "test.txt")
	if err == nil {
		t.Error("Expected file to be removed, but it still exists")
	}
}

func TestDirectoryOperations(t *testing.T) {
	memFS, server, client := newTestServer()
	defer server.Close()

	// Create a directory
	if err := client.Mkdir("testdir", 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Verify directory exists in memfs
	info, err := fs.Stat(memFS, "testdir")
	if err != nil {
		t.Fatalf("Directory not found in memfs: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected directory, got file")
	}

	// Create files in the directory
	file1, _ := client.CreateContext(context.Background(), "testdir/file1.txt", []byte("File 1"), 0644)
	file1.Close()
	file2, _ := client.CreateContext(context.Background(), "testdir/file2.txt", []byte("File 2"), 0644)
	file2.Close()

	// Read directory
	entries, err := client.ReadDir("testdir")
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}

	// Verify entry names
	names := make(map[string]bool)
	for _, entry := range entries {
		names[entry.Name()] = true
	}
	if !names["file1.txt"] || !names["file2.txt"] {
		t.Error("Missing expected files in directory listing")
	}
}

func TestRename(t *testing.T) {
	memFS, server, client := newTestServer()
	defer server.Close()

	ctx := context.Background()

	// Create a file
	file, _ := client.CreateContext(ctx, "oldname.txt", []byte("test content"), 0644)
	file.Close()

	// Rename it
	if err := client.Rename("oldname.txt", "newname.txt"); err != nil {
		t.Fatalf("Failed to rename file: %v", err)
	}

	// Verify old name doesn't exist
	_, err := fs.Stat(memFS, "oldname.txt")
	if err == nil {
		t.Error("Old file should not exist after rename")
	}

	// Verify new name exists
	info, err := fs.Stat(memFS, "newname.txt")
	if err != nil {
		t.Fatalf("New file not found: %v", err)
	}
	if info.Size() != 12 {
		t.Errorf("Expected size 12, got %d", info.Size())
	}
}

func TestChmod(t *testing.T) {
	_, server, client := newTestServer()
	defer server.Close()

	ctx := context.Background()

	// Create a file
	file, _ := client.CreateContext(ctx, "test.txt", []byte("test"), 0644)
	file.Close()

	// Change permissions
	if err := client.Chmod("test.txt", 0755); err != nil {
		t.Fatalf("Failed to chmod: %v", err)
	}

	// Verify permissions changed
	info, err := client.Stat("test.txt")
	if err != nil {
		t.Fatalf("Failed to stat: %v", err)
	}
	if info.Mode()&0777 != 0755 {
		t.Errorf("Expected mode 0755, got %o", info.Mode()&0777)
	}
}

func TestChtimes(t *testing.T) {
	_, server, client := newTestServer()
	defer server.Close()

	ctx := context.Background()

	// Create a file
	file, _ := client.CreateContext(ctx, "test.txt", []byte("test"), 0644)
	file.Close()

	// Change modification time
	newTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := client.Chtimes("test.txt", newTime, newTime); err != nil {
		t.Fatalf("Failed to chtimes: %v", err)
	}

	// Verify time changed
	info, err := client.Stat("test.txt")
	if err != nil {
		t.Fatalf("Failed to stat: %v", err)
	}
	if !info.ModTime().Equal(newTime) {
		t.Errorf("Expected time %v, got %v", newTime, info.ModTime())
	}
}

func TestSymlink(t *testing.T) {
	memFS, server, client := newTestServer()
	defer server.Close()

	ctx := context.Background()

	// Create a target file
	file, _ := client.CreateContext(ctx, "target.txt", []byte("target content"), 0644)
	file.Close()

	// Create symlink
	if err := client.Symlink("target.txt", "link.txt"); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Verify symlink exists in memfs (use WithNoFollow to not follow symlinks)
	info, err := fs.StatContext(fs.WithNoFollow(context.Background()), memFS, "link.txt")
	if err != nil {
		t.Fatalf("Symlink not found: %v", err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Error("Expected symlink")
	}

	// Read the symlink target
	target, err := client.Readlink("link.txt")
	if err != nil {
		t.Fatalf("Failed to readlink: %v", err)
	}
	if target != "target.txt" {
		t.Errorf("Expected target 'target.txt', got '%s'", target)
	}
}

func TestNotFound(t *testing.T) {
	_, server, client := newTestServer()
	defer server.Close()

	// Try to stat non-existent file
	_, err := client.Stat("nonexistent.txt")
	if err != fs.ErrNotExist {
		t.Errorf("Expected ErrNotExist, got %v", err)
	}

	// Try to open non-existent file
	_, err = client.Open("nonexistent.txt")
	if err != fs.ErrNotExist {
		t.Errorf("Expected ErrNotExist, got %v", err)
	}

	// Try to remove non-existent file
	err = client.Remove("nonexistent.txt")
	if err != fs.ErrNotExist {
		t.Errorf("Expected ErrNotExist, got %v", err)
	}
}

func TestMultipleFiles(t *testing.T) {
	_, server, client := newTestServer()
	defer server.Close()

	ctx := context.Background()

	// Create multiple files
	for i := 0; i < 10; i++ {
		filename := fmt.Sprintf("file%d.txt", i)
		content := []byte(fmt.Sprintf("Content %d", i))
		file, err := client.CreateContext(ctx, filename, content, 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", filename, err)
		}
		file.Close()
	}

	// Read directory
	entries, err := client.ReadDir(".")
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	if len(entries) != 10 {
		t.Errorf("Expected 10 files, got %d", len(entries))
	}

	// Verify each file
	for i := 0; i < 10; i++ {
		filename := fmt.Sprintf("file%d.txt", i)
		content, err := fs.ReadFile(client, filename)
		if err != nil {
			t.Errorf("Failed to read %s: %v", filename, err)
		}
		expected := fmt.Sprintf("Content %d", i)
		if string(content) != expected {
			t.Errorf("Expected '%s', got '%s'", expected, string(content))
		}
	}
}

func TestNestedDirectories(t *testing.T) {
	_, server, client := newTestServer()
	defer server.Close()

	ctx := context.Background()

	// Create nested directories
	if err := client.Mkdir("level1", 0755); err != nil {
		t.Fatalf("Failed to create level1: %v", err)
	}
	if err := client.Mkdir("level1/level2", 0755); err != nil {
		t.Fatalf("Failed to create level2: %v", err)
	}
	if err := client.Mkdir("level1/level2/level3", 0755); err != nil {
		t.Fatalf("Failed to create level3: %v", err)
	}

	// Create a file deep in the tree
	file, err := client.CreateContext(ctx, "level1/level2/level3/deep.txt", []byte("deep content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create deep file: %v", err)
	}
	file.Close()

	// Read the file
	content, err := fs.ReadFile(client, "level1/level2/level3/deep.txt")
	if err != nil {
		t.Fatalf("Failed to read deep file: %v", err)
	}
	if string(content) != "deep content" {
		t.Errorf("Expected 'deep content', got '%s'", string(content))
	}

	// List directory at each level
	entries1, _ := client.ReadDir("level1")
	if len(entries1) != 1 || entries1[0].Name() != "level2" {
		t.Error("level1 should contain level2")
	}

	entries2, _ := client.ReadDir("level1/level2")
	if len(entries2) != 1 || entries2[0].Name() != "level3" {
		t.Error("level2 should contain level3")
	}

	entries3, _ := client.ReadDir("level1/level2/level3")
	if len(entries3) != 1 || entries3[0].Name() != "deep.txt" {
		t.Error("level3 should contain deep.txt")
	}
}

func TestWriteFile(t *testing.T) {
	memFS, server, client := newTestServer()
	defer server.Close()

	// Write a file using WriteFile
	data := []byte("Hello from WriteFile!")
	err := client.WriteFile("writefile.txt", data, 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Verify file exists in memfs
	info, err := fs.StatContext(fs.WithNoFollow(context.Background()), memFS, "writefile.txt")
	if err != nil {
		t.Fatalf("File not found in memfs: %v", err)
	}
	if info.Size() != int64(len(data)) {
		t.Errorf("Expected size %d, got %d", len(data), info.Size())
	}
	if info.Mode()&0777 != 0644 {
		t.Errorf("Expected mode 0644, got %o", info.Mode()&0777)
	}

	// Read back and verify content
	content, err := fs.ReadFile(client, "writefile.txt")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("Expected content '%s', got '%s'", string(data), string(content))
	}

	// Overwrite the file
	newData := []byte("Updated content!")
	err = client.WriteFile("writefile.txt", newData, 0755)
	if err != nil {
		t.Fatalf("WriteFile (overwrite) failed: %v", err)
	}

	// Verify updated content
	content, err = fs.ReadFile(client, "writefile.txt")
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}
	if string(content) != string(newData) {
		t.Errorf("Expected content '%s', got '%s'", string(newData), string(content))
	}
}
