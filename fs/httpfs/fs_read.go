package httpfs

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"tractor.dev/wanix/fs"
)

// Open opens the named file for reading
func (fsys *FS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

// OpenContext opens the named file for reading with context
func (fsys *FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys.log.Debug("Open", "name", name)
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := fsys.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fs.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	node, err := ParseNode(fsys, name, resp.Header, content)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// Stat returns file information
func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	return fsys.StatContext(context.Background(), name)
}

// StatContext performs a HEAD request to get file metadata
func (fsys *FS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	fsys.log.Debug("Stat", "name", name)
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := fsys.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fs.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	node, err := ParseNode(fsys, name, resp.Header, nil)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// ReadDir reads the named directory and returns a list of directory entries
func (fsys *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fsys.ReadDirContext(context.Background(), name)
}

// ReadDirContext reads the named directory with context
func (fsys *FS) ReadDirContext(ctx context.Context, name string) ([]fs.DirEntry, error) {
	fsys.log.Debug("ReadDir", "name", name)
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := fsys.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fs.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	node, err := ParseNode(fsys, name, resp.Header, content)
	if err != nil {
		return nil, err
	}

	if !node.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}

	// Convert fileNode entries to fs.DirEntry
	entries := make([]fs.DirEntry, len(node.Entries()))
	for i, entry := range node.Entries() {
		entries[i] = entry
	}
	return entries, nil
}

func (fsys *FS) Readlink(name string) (string, error) {
	return fsys.ReadlinkContext(context.Background(), name)
}

func (fsys *FS) ReadlinkContext(ctx context.Context, name string) (string, error) {
	fsys.log.Debug("Readlink", "name", name)

	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := fsys.doRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	if resp.Header.Get("Content-Type") != "application/x-symlink" {
		return "", fmt.Errorf("expected Content-Type application/x-symlink, got %s", resp.Header.Get("Content-Type"))
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (fsys *FS) ReadFile(name string) ([]byte, error) {
	return fsys.ReadFileContext(context.Background(), name)
}

func (fsys *FS) ReadFileContext(ctx context.Context, name string) ([]byte, error) {
	fsys.log.Debug("ReadFile", "name", name)

	url := fsys.buildURL(name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := fsys.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return content, nil
}
