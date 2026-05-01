//go:build js && wasm

package jsfs

import (
	"path"
	"syscall"
	"syscall/js"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

type listMode int

const (
	listObjectKeys listMode = iota
	listObjView
)

type jsDir struct {
	name string
	val  js.Value
	mode listMode
	iter *fskit.DirIter
}

func newDir(displayName string, v js.Value, m listMode) *jsDir {
	if displayName == "" || displayName == "/" {
		displayName = "."
	}
	d := &jsDir{name: displayName, val: v, mode: m}
	d.iter = fskit.NewDirIter(func() ([]fs.DirEntry, error) {
		switch m {
		case listObjView:
			return listObjViewEntries(d.val), nil
		default:
			return listDirEntries(d.val), nil
		}
	})
	return d
}

func (d *jsDir) Stat() (fs.FileInfo, error) {
	return fskit.Entry(path.Base(d.name), fs.ModeDir|0555), nil
}

func (d *jsDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: syscall.EISDIR}
}

func (d *jsDir) ReadDir(n int) ([]fs.DirEntry, error) {
	return d.iter.ReadDir(n)
}

func (d *jsDir) Write([]byte) (int, error) {
	return 0, &fs.PathError{Op: "write", Path: d.name, Err: syscall.EISDIR}
}

func (d *jsDir) Close() error { return nil }

func (d *jsDir) Seek(int64, int) (int64, error) {
	return 0, &fs.PathError{Op: "seek", Path: d.name, Err: fs.ErrInvalid}
}

var _ fs.ReadDirFile = (*jsDir)(nil)
