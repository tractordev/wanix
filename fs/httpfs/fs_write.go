package httpfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"tractor.dev/wanix/fs"
)

func (fsys *FS) Patch(ctx context.Context, name string, tarBuf bytes.Buffer) error {
	fsys.log.Debug("Patch", "name", name)
	url := fsys.buildURL(name)
	req, err := http.NewRequestWithContext(context.Background(), "PATCH", url, &tarBuf)
	if err != nil {
		return err
	}

	// Set directory headers
	req.Header.Set("Content-Type", "application/x-tar")
	req.Header.Set("Change-Timestamp", strconv.FormatInt(time.Now().UnixMicro(), 10))

	resp, err := fsys.doRequest(req)
	if err != nil {
		return err
	}
	io.Copy(os.Stdout, resp.Body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// WriteFile writes data to the named file, creating it if necessary
func (fsys *FS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return fsys.WriteFileContext(context.Background(), name, data, perm, time.Now())
}

// WriteFileContext writes data to the named file with context
func (fsys *FS) WriteFileContext(ctx context.Context, name string, data []byte, perm fs.FileMode, modTime time.Time) error {
	fsys.log.Debug("WriteFile", "name", name, "size", len(data))
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	// Set headers according to the HTTP filesystem design
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Mode", formatMode(perm&^fs.ModeType)) // Remove any type bits, force regular file
	req.Header.Set("Content-Modified", strconv.FormatInt(modTime.Unix(), 10))
	req.Header.Set("Content-Ownership", "0:0")
	req.Header.Set("Content-Length", strconv.Itoa(len(data)))
	req.Header.Set("Change-Timestamp", strconv.FormatInt(time.Now().UnixMicro(), 10))

	resp, err := fsys.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Create creates or truncates the named file
func (fsys *FS) Create(name string) (fs.File, error) {
	return fsys.CreateContext(context.Background(), name, nil, 0644)
}

// CreateContext is a helper for creating files with content and mode
func (fsys *FS) CreateContext(ctx context.Context, name string, content []byte, mode fs.FileMode) (fs.File, error) {
	fsys.log.Debug("Create", "name", name)

	if err := fsys.WriteFileContext(ctx, name, content, mode, time.Now()); err != nil {
		return nil, err
	}

	return &Node{
		path:    name,
		size:    int64(len(content)),
		mode:    mode,
		modTime: time.Now(),
		isDir:   false,
		content: content,
		fs:      fsys,
	}, nil
}

func (fsys *FS) Symlink(oldname, newname string) error {
	return fsys.SymlinkContext(context.Background(), oldname, newname)
}

func (fsys *FS) SymlinkContext(ctx context.Context, oldname, newname string) error {
	fsys.log.Debug("Symlink", "oldname", oldname, "newname", newname)
	url := fsys.buildURL(newname)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader([]byte(oldname)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-symlink")
	req.Header.Set("Content-Mode", formatMode(fs.ModeSymlink|0777))
	req.Header.Set("Content-Modified", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("Content-Ownership", "0:0")
	req.Header.Set("Change-Timestamp", strconv.FormatInt(time.Now().UnixMicro(), 10))

	resp, err := fsys.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

func (fsys *FS) Rename(oldname, newname string) error {
	return fsys.RenameContext(context.Background(), oldname, newname)
}

func (fsys *FS) RenameContext(ctx context.Context, oldname, newname string) error {
	fsys.log.Debug("Rename", "oldname", oldname, "newname", newname)
	url := fsys.buildURL(oldname)

	req, err := http.NewRequestWithContext(ctx, "MOVE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Destination", "/"+newname)

	resp, err := fsys.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Mkdir creates a directory
func (fsys *FS) Mkdir(name string, perm fs.FileMode) error {
	return fsys.MkdirContext(context.Background(), name, perm)
}

// MkdirContext creates a directory with context
func (fsys *FS) MkdirContext(ctx context.Context, name string, perm fs.FileMode) error {
	fsys.log.Debug("Mkdir", "name", name)
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader([]byte{}))
	if err != nil {
		return err
	}

	// Set directory headers
	req.Header.Set("Content-Type", "application/x-directory")
	req.Header.Set("Content-Mode", formatMode(perm|fs.ModeDir))
	req.Header.Set("Content-Modified", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("Content-Ownership", "0:0")
	req.Header.Set("Content-Length", "0")
	req.Header.Set("Change-Timestamp", strconv.FormatInt(time.Now().UnixMicro(), 10))

	resp, err := fsys.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Remove removes a file or empty directory
func (fsys *FS) Remove(name string) error {
	return fsys.RemoveContext(context.Background(), name)
}

// RemoveContext removes a file or directory with context
func (fsys *FS) RemoveContext(ctx context.Context, name string) error {
	fsys.log.Debug("Remove", "name", name)
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := fsys.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fs.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Chmod changes the mode of the named file
func (fsys *FS) Chmod(name string, mode fs.FileMode) error {
	return fsys.ChmodContext(context.Background(), name, mode)
}

// ChmodContext changes file mode with context
func (fsys *FS) ChmodContext(ctx context.Context, name string, mode fs.FileMode) error {
	fsys.log.Debug("Chmod", "name", name, "mode", mode)
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Mode", formatMode(mode))
	req.Header.Set("Change-Timestamp", strconv.FormatInt(time.Now().UnixMicro(), 10))

	resp, err := fsys.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fs.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Chown changes the numeric uid and gid of the named file
func (fsys *FS) Chown(name string, uid, gid int) error {
	return fsys.ChownContect(context.Background(), name, uid, gid)
}

// ChownContect changes ownership with context
func (fsys *FS) ChownContect(ctx context.Context, name string, uid, gid int) error {
	fsys.log.Debug("Chown", "name", name, "uid", uid, "gid", gid)
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, nil)
	if err != nil {
		return err
	}

	ownership := fmt.Sprintf("%d:%d", uid, gid)
	req.Header.Set("Content-Ownership", ownership)
	req.Header.Set("Change-Timestamp", strconv.FormatInt(time.Now().UnixMicro(), 10))

	resp, err := fsys.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fs.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// Chtimes changes the access and modification times of the named file
func (fsys *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return fsys.ChtimesContext(context.Background(), name, atime, mtime)
}

// ChtimesContext changes times with context
func (fsys *FS) ChtimesContext(ctx context.Context, name string, atime time.Time, mtime time.Time) error {
	fsys.log.Debug("Chtimes", "name", name, "atime", atime, "mtime", mtime)
	url := fsys.buildURL(name)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, nil)
	if err != nil {
		return err
	}

	// HTTP filesystem design only supports modification time
	req.Header.Set("Content-Modified", strconv.FormatInt(mtime.Unix(), 10))
	req.Header.Set("Change-Timestamp", strconv.FormatInt(time.Now().UnixMicro(), 10))

	resp, err := fsys.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fs.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}
