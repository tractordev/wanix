package httpfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
		return pathError("patch", name, resp)
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
		return pathError("write", name, resp)
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

// OpenFile opens a file with the given flags.
func (fsys *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	return fsys.OpenFileContext(context.Background(), name, flag, perm)
}

// OpenFileContext opens a file with the given flags and context.
func (fsys *FS) OpenFileContext(ctx context.Context, name string, flag int, perm fs.FileMode) (fs.File, error) {
	if fsys.shouldIgnore(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if perm == 0 {
		perm = 0644
	}
	if flag&(os.O_WRONLY|os.O_RDWR) != 0 {
		_, err := fsys.StatContext(ctx, name)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
			if flag&os.O_CREATE == 0 {
				return nil, err
			}
			return fsys.CreateContext(ctx, name, nil, perm)
		}
		if flag&os.O_TRUNC != 0 {
			if err := fsys.WriteFileContext(ctx, name, nil, perm, time.Now()); err != nil {
				return nil, err
			}
			return &Node{
				path:    name,
				size:    0,
				mode:    perm,
				modTime: time.Now(),
				fs:      fsys,
			}, nil
		}
		n, err := fsys.OpenContext(ctx, name)
		if err != nil {
			return nil, err
		}
		if flag&os.O_APPEND != 0 {
			if _, err := fs.Seek(n, 0, io.SeekEnd); err != nil {
				_ = n.Close()
				return nil, err
			}
		}
		return n, nil
	}
	return fsys.OpenContext(ctx, name)
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
		return pathError("symlink", newname, resp)
	}

	return nil
}

func (fsys *FS) Rename(oldname, newname string) error {
	return fsys.RenameContext(context.Background(), oldname, newname)
}

func (fsys *FS) RenameContext(ctx context.Context, oldname, newname string) error {
	fsys.log.Debug("Rename", "oldname", oldname, "newname", newname)
	u := fsys.buildURL(oldname)

	req, err := http.NewRequestWithContext(ctx, "MOVE", u, nil)
	if err != nil {
		return err
	}

	dest, err := url.Parse(fsys.buildURL(newname))
	if err != nil {
		return err
	}
	req.Header.Set("Destination", dest.Path)

	resp, err := fsys.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return pathError("rename", oldname, resp)
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
		return pathError("mkdir", name, resp)
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

	if resp.StatusCode != http.StatusOK {
		return pathError("remove", name, resp)
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

	if resp.StatusCode != http.StatusOK {
		return pathError("chmod", name, resp)
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

	if resp.StatusCode != http.StatusOK {
		return pathError("chown", name, resp)
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

	if resp.StatusCode != http.StatusOK {
		return pathError("chtimes", name, resp)
	}

	return nil
}
