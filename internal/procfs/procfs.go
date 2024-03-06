package procfs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/wanix/kernel/proc"
)

// TODO: Support searching by PID or exe name.
// E.g. `/proc/PID/$id` and `/proc/EXE/$name`, where the EXE path returns PIDs
// only for the given exe (essentially filtering PIDs by EXE name).

type FS struct {
	ps *proc.Service
}

func New(p *proc.Service) *FS {
	return &FS{ps: p}
}

func (f *FS) Create(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "create", Path: name, Err: ErrUnimplemented}
}

func (f *FS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	if name == "." {
		return &file{fs: f, isRoot: true}, nil
	}

	// TODO: hierarchical data?
	pidStr, _, hasSubpath := strings.Cut(name, "/")
	if hasSubpath {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	pid, err := strconv.ParseInt(pidStr, 0, 0)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	p, err := f.ps.Get(int(pid))
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	return &file{fs: f, proc: *p}, nil
}

func (f *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	return f.Open(name)
}

func (f *FS) Remove(name string) error {
	return &fs.PathError{Op: "remove", Path: name, Err: ErrUnimplemented}
}
func (f *FS) RemoveAll(path string) error {
	return &fs.PathError{Op: "removeall", Path: path, Err: ErrUnimplemented}
}

func (f *FS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	if name == "." {
		return &fileInfo{name: name, isDir: true}, nil
	}

	// TODO: hierarchical data?
	pidStr, _, hasSubpath := strings.Cut(name, "/")
	if hasSubpath {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	pid, err := strconv.ParseInt(pidStr, 0, 0)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}

	p, err := f.ps.Get(int(pid))
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}

	return &fileInfo{name: filepath.Base(p.Path), isDir: false}, nil
}

type file struct {
	fs     *FS
	proc   proc.Process
	isRoot bool

	buffer []byte
	offset int64
}

func (f *file) Close() error {
	clear(f.buffer)
	f.offset = 0
	return nil
}

func (f *file) Read(b []byte) (int, error) {
	if f.isRoot {
		return 0, nil
	}

	if f.buffer == nil {
		var err error
		f.buffer, err = json.MarshalIndent(f.proc, "", "    ")
		if err != nil {
			return 0, err
		}
	}

	if f.offset >= int64(len(f.buffer)) {
		return 0, io.EOF
	}

	var n int
	rest := f.buffer[f.offset:]
	if len(rest) < len(b) {
		n = len(rest)
	} else {
		n = len(b)
	}

	copy(b, rest[:n])
	f.offset += int64(n)
	return n, nil
}

func (f *file) ReadDir(n int) ([]fs.DirEntry, error) {
	// TODO: hierarchical data?
	if !f.isRoot {
		return nil, errors.ErrUnsupported
	}
	running := f.fs.ps.GetAll()

	var res []fs.DirEntry
	for pid := range running {
		res = append(res, &fileInfo{name: strconv.FormatInt(int64(pid), 10), isDir: false})
	}
	return res, nil
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.offset = offset
	case io.SeekCurrent:
		f.offset += offset
	case io.SeekEnd:
		f.offset = int64(len(f.buffer)) + offset
	}
	if f.offset < 0 {
		f.offset = 0
		return 0, fmt.Errorf("%w: resultant offset cannot be negative", fs.ErrInvalid)
	}
	return f.offset, nil
}

func (f *file) Stat() (fs.FileInfo, error) {
	return nil, ErrUnimplemented
}

type fileInfo struct {
	// base name
	name  string
	isDir bool
}

func (i *fileInfo) Name() string       { return i.name }
func (i *fileInfo) Size() int64        { return 0 }
func (i *fileInfo) Mode() fs.FileMode  { return 0444 }
func (i *fileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (i *fileInfo) IsDir() bool        { return i.isDir }
func (i *fileInfo) Sys() any           { return nil }

// These allow it to act as DirEntry as well

func (i *fileInfo) Info() (fs.FileInfo, error) { return i, nil }
func (i *fileInfo) Type() fs.FileMode          { return i.Mode() }

// These functions won't be supported

func (f *FS) Chmod(name string, mode fs.FileMode) error {
	return &fs.PathError{Op: "chmod", Path: name, Err: errors.ErrUnsupported}
}
func (f *FS) Chown(name string, uid, gid int) error {
	return &fs.PathError{Op: "chown", Path: name, Err: errors.ErrUnsupported}
}
func (f *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return &fs.PathError{Op: "chtimes", Path: name, Err: errors.ErrUnsupported}
}
func (f *FS) Mkdir(name string, perm fs.FileMode) error {
	return &fs.PathError{Op: "mkdir", Path: name, Err: errors.ErrUnsupported}
}
func (f *FS) MkdirAll(path string, perm fs.FileMode) error {
	return &fs.PathError{Op: "mkdirall", Path: path, Err: errors.ErrUnsupported}
}
func (f *FS) Rename(oldname, newname string) error {
	return &fs.PathError{Op: "rename", Path: oldname, Err: errors.ErrUnsupported}
}

var ErrUnimplemented = errors.New("Unimplemented")
