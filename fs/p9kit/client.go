package p9kit

import (
	"context"
	"io"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"

	"github.com/hugelgupf/p9/p9"
)

func ClientFS(conn net.Conn, aname string, o ...p9.ClientOpt) (fs.FS, error) {
	client, err := p9.NewClient(conn, o...)
	if err != nil {
		return nil, err
	}

	var root p9.File
	root, err = client.Attach(aname)
	if err != nil {
		return nil, err
	}

	return &FS{client: client, root: root}, nil
}

type FS struct {
	client *p9.Client
	root   p9.File
}

func walkParts(name string) []string {
	name = path.Clean(name)
	if name == "." {
		return nil
	}
	return strings.Split(name, "/")
}

func (fsys *FS) walk(name string) (p9.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "walk", Path: name, Err: fs.ErrInvalid}
	}
	_, f, err := fsys.root.Walk(walkParts(name))
	if err != nil {
		return nil, translateError("walk", name, err)
	}
	return f, nil
}

func (fsys *FS) Open(name string) (fs.File, error) {
	return fsys.OpenContext(context.Background(), name)
}

func (fsys *FS) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	f, err := fsys.walk(name)
	if err != nil {
		return nil, err
	}

	// Try to open with read-write first
	_, _, err = f.Open(p9.ReadWrite)
	if err != nil {
		// Fallback to read-only (might be a dir or read-only file)
		_, _, err = f.Open(p9.ReadOnly)
		if err != nil {
			return nil, translateError("open", name, err)
		}
	}

	return &remoteFile{
		file: f,
		root: fsys.root,
		name: path.Base(name),
		path: walkParts(name),
	}, nil
}

func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	return fsys.StatContext(context.Background(), name)
}

func (fsys *FS) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	f, err := fsys.walk(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return fileInfo(f, path.Base(name))
}

func (fsys *FS) Create(name string) (fs.File, error) {
	if !fs.ValidPath(name) || name == "." {
		return nil, &fs.PathError{Op: "create", Path: name, Err: fs.ErrInvalid}
	}

	d, err := fsys.walk(path.Dir(name))
	if err != nil {
		return nil, err
	}

	basename := path.Base(name)

	// Try to create the file
	_, _, _, err = d.Create(basename, p9.ReadWrite, p9.FileMode(0644), 0, 0)
	d.Close() // Close directory after creating file

	// Walk to the file (whether created or existed) and open it
	f, err := fsys.walk(name)
	if err != nil {
		return nil, err
	}

	// Truncate to 0 (handles both new file and truncating existing)
	if err := f.SetAttr(p9.SetAttrMask{Size: true}, p9.SetAttr{Size: 0}); err != nil {
		f.Close()
		return nil, translateError("create", name, err)
	}

	// Open for read-write
	_, _, err = f.Open(p9.ReadWrite)
	if err != nil {
		f.Close()
		return nil, translateError("create", name, err)
	}

	return &remoteFile{
		file: f,
		root: fsys.root,
		name: basename,
		path: walkParts(name),
	}, nil
}

func (fsys *FS) Mkdir(name string, perm fs.FileMode) error {
	if !fs.ValidPath(name) || name == "." {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrInvalid}
	}

	d, err := fsys.walk(path.Dir(name))
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = d.Mkdir(path.Base(name), p9.FileMode(perm), 0, 0)
	return translateError("mkdir", name, err)
}

func (fsys *FS) Remove(name string) error {
	if !fs.ValidPath(name) || name == "." {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
	}

	// Check if it's a directory
	info, err := fsys.Stat(name)
	if err != nil {
		return err
	}

	d, err := fsys.walk(path.Dir(name))
	if err != nil {
		return err
	}
	defer d.Close()

	// Use AT_REMOVEDIR flag for directories
	flags := uint32(0)
	if info.IsDir() {
		flags = 0x200 // AT_REMOVEDIR
	}

	err = d.UnlinkAt(path.Base(name), flags)
	return translateError("remove", name, err)
}

func (fsys *FS) Rename(oldpath, newpath string) error {
	if !fs.ValidPath(oldpath) || !fs.ValidPath(newpath) {
		return &fs.PathError{Op: "rename", Path: oldpath, Err: fs.ErrInvalid}
	}

	oldDir, err := fsys.walk(path.Dir(oldpath))
	if err != nil {
		return err
	}
	defer oldDir.Close()

	newDir, err := fsys.walk(path.Dir(newpath))
	if err != nil {
		return err
	}
	defer newDir.Close()

	err = oldDir.RenameAt(path.Base(oldpath), newDir, path.Base(newpath))
	return translateError("rename", oldpath, err)
}

func (fsys *FS) Chmod(name string, mode fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chmod", Path: name, Err: fs.ErrInvalid}
	}

	f, err := fsys.walk(name)
	if err != nil {
		return err
	}
	defer f.Close()

	err = f.SetAttr(p9.SetAttrMask{Permissions: true}, p9.SetAttr{Permissions: p9.FileMode(mode)})
	return translateError("chmod", name, err)
}

func (fsys *FS) Chown(name string, uid, gid int) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chown", Path: name, Err: fs.ErrInvalid}
	}

	f, err := fsys.walk(name)
	if err != nil {
		return err
	}
	defer f.Close()

	mask := p9.SetAttrMask{}
	attr := p9.SetAttr{}

	if uid >= 0 {
		mask.UID = true
		attr.UID = p9.UID(uid)
	}
	if gid >= 0 {
		mask.GID = true
		attr.GID = p9.GID(gid)
	}

	err = f.SetAttr(mask, attr)
	return translateError("chown", name, err)
}

func (fsys *FS) Chtimes(name string, atime, mtime time.Time) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chtimes", Path: name, Err: fs.ErrInvalid}
	}

	f, err := fsys.walk(name)
	if err != nil {
		return err
	}
	defer f.Close()

	mask := p9.SetAttrMask{
		ATime:              true,
		ATimeNotSystemTime: true,
		MTime:              true,
		MTimeNotSystemTime: true,
	}
	attr := p9.SetAttr{
		ATimeSeconds:     uint64(atime.Unix()),
		ATimeNanoSeconds: uint64(atime.Nanosecond()),
		MTimeSeconds:     uint64(mtime.Unix()),
		MTimeNanoSeconds: uint64(mtime.Nanosecond()),
	}

	err = f.SetAttr(mask, attr)
	return translateError("chtimes", name, err)
}

func (fsys *FS) Truncate(name string, size int64) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "truncate", Path: name, Err: fs.ErrInvalid}
	}

	f, err := fsys.walk(name)
	if err != nil {
		return err
	}
	defer f.Close()

	err = f.SetAttr(p9.SetAttrMask{Size: true}, p9.SetAttr{Size: uint64(size)})
	return translateError("truncate", name, err)
}

func (fsys *FS) Symlink(oldname, newname string) error {
	if !fs.ValidPath(newname) {
		return &fs.PathError{Op: "symlink", Path: newname, Err: fs.ErrInvalid}
	}

	d, err := fsys.walk(path.Dir(newname))
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = d.Symlink(oldname, path.Base(newname), 0, 0)
	return translateError("symlink", newname, err)
}

func (fsys *FS) Readlink(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}

	f, err := fsys.walk(name)
	if err != nil {
		return "", err
	}
	defer f.Close()

	target, err := f.Readlink()
	if err != nil {
		return "", translateError("readlink", name, err)
	}
	return target, nil
}

func (fsys *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}

	f, err := fsys.walk(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	_, _, err = f.Open(p9.ReadOnly)
	if err != nil {
		return nil, translateError("readdir", name, err)
	}

	dirents, err := f.Readdir(0, 65535) // max uint32 breaks p9kit server
	if err != nil {
		return nil, translateError("readdir", name, err)
	}

	entries := make([]fs.DirEntry, 0, len(dirents))
	for _, entry := range dirents {
		// To fully mimic os.DirFS semantics, we can Walk and Stat, but for now just minimal implementation:
		fi, err := fileInfo(f, entry.Name)
		if err != nil {
			continue // Skip entries we can't stat
		}
		if dirEntry, ok := fi.(fs.DirEntry); ok {
			entries = append(entries, dirEntry)
		}
	}
	return entries, nil
}

type remoteFile struct {
	name   string
	file   p9.File
	root   p9.File
	path   []string
	offset int64
	iter   *fskit.DirIter
}

func (f *remoteFile) Read(p []byte) (n int, err error) {
	n, err = f.file.ReadAt(p, f.offset)
	if err != nil && err != io.EOF {
		return n, err
	}
	f.offset += int64(n)
	return n, err
}

func (f *remoteFile) Write(p []byte) (n int, err error) {
	n, err = f.file.WriteAt(p, f.offset)
	if err != nil {
		return n, err
	}
	f.offset += int64(n)
	return n, nil
}

func (f *remoteFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		// offset is already the position
	case io.SeekCurrent:
		offset += f.offset
	case io.SeekEnd:
		// Get file size
		fi, err := f.Stat()
		if err != nil {
			return 0, err
		}
		offset += fi.Size()
	default:
		return 0, &fs.PathError{Op: "seek", Path: f.name, Err: fs.ErrInvalid}
	}

	if offset < 0 {
		return 0, &fs.PathError{Op: "seek", Path: f.name, Err: fs.ErrInvalid}
	}

	f.offset = offset
	return offset, nil
}

func (f *remoteFile) ReadAt(p []byte, off int64) (n int, err error) {
	return f.file.ReadAt(p, off)
}

func (f *remoteFile) WriteAt(p []byte, off int64) (n int, err error) {
	return f.file.WriteAt(p, off)
}

func (f *remoteFile) Close() error {
	if err := f.file.FSync(); err != nil {
		return err
	}
	return f.file.Close()
}

func (f *remoteFile) Stat() (fs.FileInfo, error) {
	return fileInfo(f.file, f.name)
}

func (f *remoteFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if f.iter == nil {
		f.iter = fskit.NewDirIter(func() ([]fs.DirEntry, error) {
			dirents, err := f.file.Readdir(0, 65535) // max uint32 breaks p9kit server
			if err != nil {
				return nil, translateError("readdir", f.name, err)
			}

			entries := make([]fs.DirEntry, 0, len(dirents))
			for _, entry := range dirents {
				// To fully mimic os.DirFS semantics, we can Walk and Stat, but for now just minimal implementation:
				fi, err := fileInfo(f.file, entry.Name)
				if err != nil {
					continue // Skip entries we can't stat
				}
				if dirEntry, ok := fi.(fs.DirEntry); ok {
					entries = append(entries, dirEntry)
				}
			}
			return entries, nil
		})
	}
	return f.iter.ReadDir(n)
}

func fileInfo(f p9.File, name string) (fs.FileInfo, error) {
	_, _, attr, err := f.GetAttr(p9.AttrMask{
		Mode:  true,
		ATime: true,
		MTime: true,
		CTime: true,
		Size:  true,
	})
	if err != nil {
		return nil, err
	}

	// Extract only permission bits from p9 mode, then add Go file type bits
	// Must mask with S_IFMT to get file type, then compare (not just AND check)
	var mode fs.FileMode = fs.FileMode(attr.Mode & 0o777)
	fileType := attr.Mode & 0o170000 // S_IFMT mask
	if fileType == p9.ModeDirectory {
		mode |= fs.ModeDir
	}
	if fileType == p9.ModeSymlink {
		mode |= fs.ModeSymlink
	}
	// Regular files (S_IFREG = 0o100000) get no special mode bit

	return fskit.Entry(
		name,
		mode,
		int64(attr.Size),
		time.Unix(int64(attr.MTimeSeconds), int64(attr.MTimeNanoSeconds)),
	), nil
}

// translateError converts p9 errors to fs errors with proper context
func translateError(op, path string, err error) error {
	if err == nil {
		return nil
	}

	// Check for common error strings
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "file exists"):
		return &fs.PathError{Op: op, Path: path, Err: fs.ErrExist}
	case strings.Contains(errStr, "no such file") || strings.Contains(errStr, "not found"):
		return &fs.PathError{Op: op, Path: path, Err: fs.ErrNotExist}
	case strings.Contains(errStr, "permission denied"):
		return &fs.PathError{Op: op, Path: path, Err: fs.ErrPermission}
	case strings.Contains(errStr, "invalid") || strings.Contains(errStr, "bad"):
		return &fs.PathError{Op: op, Path: path, Err: fs.ErrInvalid}
	}

	// Check if it's already an os error we can recognize
	if os.IsNotExist(err) {
		return &fs.PathError{Op: op, Path: path, Err: fs.ErrNotExist}
	}
	if os.IsExist(err) {
		return &fs.PathError{Op: op, Path: path, Err: fs.ErrExist}
	}
	if os.IsPermission(err) {
		return &fs.PathError{Op: op, Path: path, Err: fs.ErrPermission}
	}

	// Return as-is but wrapped in PathError for consistency
	return &fs.PathError{Op: op, Path: path, Err: err}
}

// isExistsError checks if an error indicates the file already exists
func isExistsError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "file exists") || os.IsExist(err)
}
