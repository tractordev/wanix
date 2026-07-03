package httpfs

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/localfs"
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

func TestBuildURLWithQuery(t *testing.T) {
	fsys := New("http://localhost:8787/~/?token=abc", nil)
	if got := fsys.buildURL("token"); got != "http://localhost:8787/~/token?token=abc" {
		t.Fatalf("buildURL(token) = %q, want %q", got, "http://localhost:8787/~/token?token=abc")
	}
	if got := fsys.buildURL("."); got != "http://localhost:8787/~/?token=abc" {
		t.Fatalf("buildURL(.) = %q, want %q", got, "http://localhost:8787/~/?token=abc")
	}
}

func TestOpenFileCreateWrite(t *testing.T) {
	memFS, server, client := newTestServer()
	defer server.Close()

	f, err := client.OpenFile("hello", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if _, err := fs.Write(f, []byte("hello\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	content, err := fs.ReadFile(memFS, "hello")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "hello\n" {
		t.Fatalf("got %q, want %q", content, "hello\n")
	}
}

func TestOpenFileCreateWithQueryURL(t *testing.T) {
	memFS := memfs.New()
	server := httptest.NewServer(NewServer(memFS))
	defer server.Close()

	client := New(server.URL+"/?token=abc", nil)
	f, err := client.OpenFile("hello", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if _, err := fs.Write(f, []byte("hi")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	f.Close()

	content, err := fs.ReadFile(memFS, "hello")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "hi" {
		t.Fatalf("got %q", content)
	}
}

func TestPutWithOwnershipLocalFS(t *testing.T) {
	dir := t.TempDir()
	fsys, err := localfs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewServer(fsys))
	defer server.Close()

	client := New(server.URL, nil)
	if err := client.WriteFile("hello", []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	content, err := fs.ReadFile(fsys, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "hello" {
		t.Fatalf("got %q", content)
	}
}

func TestNotFound(t *testing.T) {
	_, server, client := newTestServer()
	defer server.Close()

	// Try to stat non-existent file
	_, err := client.Stat("nonexistent.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Expected ErrNotExist, got %v", err)
	}
	if err == fs.ErrNotExist {
		t.Error("expected *fs.PathError wrapper, got bare ErrNotExist")
	}

	// Try to open non-existent file
	_, err = client.Open("nonexistent.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Expected ErrNotExist, got %v", err)
	}

	// Try to remove non-existent file
	err = client.Remove("nonexistent.txt")
	if !errors.Is(err, fs.ErrNotExist) {
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

func TestDirectoryModeWithoutContentMode(t *testing.T) {
	memFS := memfs.New()
	inner := NewServer(memFS)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		inner.ServeHTTP(rec, r)
		for k, v := range rec.Header() {
			if k == "Content-Mode" {
				continue
			}
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		w.WriteHeader(rec.Code)
		w.Write(rec.Body.Bytes())
	}))
	defer server.Close()
	client := New(server.URL, nil)

	info, err := client.Stat(".")
	if err != nil {
		t.Fatalf("Stat root: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected root to be a directory")
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("directory missing execute bit: %o", info.Mode()&0777)
	}
}

func doRequest(t *testing.T, server *httptest.Server, method, path string, body io.Reader, headers http.Header) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, server.URL+path, body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	for k, vals := range headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return resp
}

func TestOptions(t *testing.T) {
	_, server, _ := newTestServer()
	defer server.Close()

	resp := doRequest(t, server, http.MethodOptions, "/", nil, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if allow := resp.Header.Get("Allow"); allow != protocolMethods {
		t.Errorf("Allow = %q, want %q", allow, protocolMethods)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK\n" {
		t.Errorf("body = %q, want %q", string(body), "OK\n")
	}
}

func TestCopyFile(t *testing.T) {
	memFS, server, client := newTestServer()
	defer server.Close()

	ctx := context.Background()
	file, _ := client.CreateContext(ctx, "original.txt", []byte("copy me"), 0644)
	file.Close()

	resp := doRequest(t, server, "COPY", "/original.txt", nil, http.Header{
		"Destination": {"/copy.txt"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("COPY failed: %d %s", resp.StatusCode, readBody(resp))
	}

	if _, err := fs.Stat(memFS, "original.txt"); err != nil {
		t.Fatal("source should still exist after COPY")
	}
	content, err := fs.ReadFile(memFS, "copy.txt")
	if err != nil {
		t.Fatalf("copy not found: %v", err)
	}
	if string(content) != "copy me" {
		t.Errorf("got %q", string(content))
	}
}

func TestCopyDirectory(t *testing.T) {
	memFS, server, client := newTestServer()
	defer server.Close()

	ctx := context.Background()
	if err := client.Mkdir("srcdir", 0755); err != nil {
		t.Fatal(err)
	}
	f, _ := client.CreateContext(ctx, "srcdir/nested.txt", []byte("nested"), 0644)
	f.Close()

	resp := doRequest(t, server, "COPY", "/srcdir", nil, http.Header{
		"Destination": {"/dstdir"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("COPY dir failed: %d %s", resp.StatusCode, readBody(resp))
	}

	if _, err := fs.Stat(memFS, "srcdir/nested.txt"); err != nil {
		t.Fatal("source tree should remain")
	}
	content, err := fs.ReadFile(memFS, "dstdir/nested.txt")
	if err != nil {
		t.Fatalf("copied file not found: %v", err)
	}
	if string(content) != "nested" {
		t.Errorf("got %q", string(content))
	}
}

func TestMoveCopyOverwriteFails(t *testing.T) {
	_, server, client := newTestServer()
	defer server.Close()

	ctx := context.Background()
	a, _ := client.CreateContext(ctx, "a.txt", []byte("a"), 0644)
	a.Close()
	b, _ := client.CreateContext(ctx, "b.txt", []byte("b"), 0644)
	b.Close()

	for _, method := range []string{"MOVE", "COPY"} {
		t.Run(method, func(t *testing.T) {
			resp := doRequest(t, server, method, "/a.txt", nil, http.Header{
				"Destination": {"/b.txt"},
				"Overwrite":     {"F"},
			})
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusPreconditionFailed {
				t.Fatalf("expected 412, got %d: %s", resp.StatusCode, readBody(resp))
			}
		})
	}
}

func TestRecursiveDelete(t *testing.T) {
	memFS, server, client := newTestServer()
	defer server.Close()

	ctx := context.Background()
	if err := client.Mkdir("removedir", 0755); err != nil {
		t.Fatal(err)
	}
	f, _ := client.CreateContext(ctx, "removedir/child.txt", []byte("x"), 0644)
	f.Close()

	if err := client.Remove("removedir"); err != nil {
		t.Fatalf("Remove dir: %v", err)
	}
	if _, err := fs.Stat(memFS, "removedir"); err == nil {
		t.Error("directory should be gone")
	}
	if _, err := fs.Stat(memFS, "removedir/child.txt"); err == nil {
		t.Error("child should be gone")
	}
}

func TestTarPatch(t *testing.T) {
	memFS, server, client := newTestServer()
	defer server.Close()

	ctx := context.Background()
	f, _ := client.CreateContext(ctx, "keep.txt", []byte("stay"), 0644)
	f.Close()
	f2, _ := client.CreateContext(ctx, "remove.txt", []byte("go"), 0644)
	f2.Close()

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)

	addHdr := &tar.Header{
		Name:     "added.txt",
		Mode:     0644,
		Size:     7,
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(addHdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("newfile")); err != nil {
		t.Fatal(err)
	}

	delHdr := &tar.Header{
		Name:       "remove.txt",
		Typeflag:   tar.TypeReg,
		PAXRecords: map[string]string{"delete": ""},
	}
	if err := tw.WriteHeader(delHdr); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	if err := client.Patch(ctx, ".", tarBuf); err != nil {
		t.Fatalf("Patch: %v", err)
	}

	if _, err := fs.Stat(memFS, "remove.txt"); err == nil {
		t.Error("remove.txt should be deleted")
	}
	if _, err := fs.Stat(memFS, "keep.txt"); err != nil {
		t.Error("keep.txt should remain")
	}
	content, err := fs.ReadFile(memFS, "added.txt")
	if err != nil {
		t.Fatalf("added.txt: %v", err)
	}
	if string(content) != "newfile" {
		t.Errorf("got %q", string(content))
	}
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}
