//go:build js && wasm

package idbfs

import (
	"context"
	_ "embed"
	"log"
	"syscall/js"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/pstat"
	"tractor.dev/wanix/web/jsutil"
)

//go:embed idbfs.js
var idbfsJS []byte

func BlobURL() string {
	jsBuf := js.Global().Get("Uint8Array").New(len(idbfsJS))
	js.CopyBytesToJS(jsBuf, idbfsJS)
	blob := js.Global().Get("Blob").New([]any{jsBuf}, js.ValueOf(map[string]any{"type": "text/javascript"}))
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	return url.String()
}

func Load() {
	v := js.Global().Get("IDBFS")
	if !v.IsUndefined() {
		return
	}
	p := jsutil.LoadScript(BlobURL(), true)
	_, err := jsutil.AwaitErr(p)
	if err != nil {
		panic(err)
	}
}

func New(name string) FS {
	Load()
	return FS{Value: js.Global().Get("IDBFS").New(name)}
}

func convertErr(e error) error {
	if e == nil {
		return nil
	}
	err, ok := e.(js.Error)
	if !ok {
		return e
	}
	code := err.Value.Get("code").String()
	switch code {
	case "ENOENT":
		return fs.ErrNotExist
	case "EEXIST":
		return fs.ErrExist
	case "EIO":
		return fs.ErrInvalid
	case "ENOTEMPTY":
		return fs.ErrNotEmpty
	case "EISDIR":
		return fs.ErrInvalid
	case "EBADF":
		return fs.ErrClosed
	}
	log.Println("idbfs: error", code, e)
	return e
}

type FS struct {
	js.Value
}

var _ fs.FS = FS{}
var _ fs.ReadDirFS = FS{}

func (fsys FS) Open(name string) (fs.File, error) {
	v, err := jsutil.AwaitErr(fsys.Call("open", name))
	if err != nil {
		return nil, convertErr(err)
	}
	return File{Value: v}, nil
}

func (fsys FS) Create(name string) (fs.File, error) {
	v, err := jsutil.AwaitErr(fsys.Call("create", name))
	if err != nil {
		return nil, convertErr(err)
	}
	return File{Value: v}, nil
}

func (fsys FS) OpenFile(name string, flag int, _ fs.FileMode) (fs.File, error) {
	v, err := jsutil.AwaitErr(fsys.Call("openfile", name, flag))
	if err != nil {
		return nil, convertErr(err)
	}
	return File{Value: v}, nil
}

func (fsys FS) Mkdir(name string, perm fs.FileMode) error {
	_, err := jsutil.AwaitErr(fsys.Call("mkdir", name, uint32(perm)))
	return convertErr(err)
}

func (fsys FS) Symlink(oldpath, newpath string) error {
	_, err := jsutil.AwaitErr(fsys.Call("symlink", oldpath, newpath))
	return convertErr(err)
}

func (fsys FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	_, err := jsutil.AwaitErr(fsys.Call("chtimes", name, atime.Unix(), mtime.Unix()))
	return convertErr(err)
}

func (fsys FS) Chmod(name string, mode fs.FileMode) error {
	_, err := jsutil.AwaitErr(fsys.Call("chmod", name, uint32(mode)))
	return convertErr(err)
}

func (fsys FS) StatContext(_ context.Context, name string) (fs.FileInfo, error) {
	return fsys.Stat(name)
}

func (fsys FS) Stat(name string) (fs.FileInfo, error) {
	v, err := jsutil.AwaitErr(fsys.Call("stat", name))
	if err != nil {
		return nil, convertErr(err)
	}
	return FileInfo{Value: v}, nil
}

func (fsys FS) Truncate(name string, size int64) error {
	_, err := jsutil.AwaitErr(fsys.Call("truncate", name, size))
	return convertErr(err)
}

func (fsys FS) Remove(name string) error {
	_, err := jsutil.AwaitErr(fsys.Call("remove", name))
	return convertErr(err)
}

func (fsys FS) Rename(oldpath, newpath string) error {
	_, err := jsutil.AwaitErr(fsys.Call("rename", oldpath, newpath))
	return convertErr(err)
}

func (fsys FS) ReadDir(name string) ([]fs.DirEntry, error) {
	v, err := jsutil.AwaitErr(fsys.Call("readdir", name))
	if err != nil {
		return nil, convertErr(err)
	}
	var entries []fs.DirEntry
	for i := 0; i < v.Length(); i++ {
		entries = append(entries, FileInfo{Value: v.Index(i)})
	}
	return entries, nil
}

func (fsys FS) Readlink(name string) (string, error) {
	v, err := jsutil.AwaitErr(fsys.Call("readlink", name))
	if err != nil {
		return "", convertErr(err)
	}
	return v.String(), nil
}

type File struct {
	js.Value
}

var _ fs.File = File{}
var _ fs.ReadDirFile = File{}

func (f File) Close() error {
	_, err := jsutil.AwaitErr(f.Call("close"))
	return convertErr(err)
}

func (f File) Stat() (fs.FileInfo, error) {
	v, err := jsutil.AwaitErr(f.Call("stat"))
	if err != nil {
		return nil, convertErr(err)
	}
	return FileInfo{Value: v}, nil
}

func (f File) Read(p []byte) (int, error) {
	r := &jsutil.Reader{Value: f.Value}
	return r.Read(p)
}

func (f File) Write(p []byte) (int, error) {
	w := &jsutil.Writer{Value: f.Value}
	return w.Write(p)
}

func (f File) Seek(offset int64, whence int) (int64, error) {
	v, err := jsutil.AwaitErr(f.Call("seek", offset, whence))
	if err != nil {
		return 0, convertErr(err)
	}
	return int64(v.Int()), nil
}

func (f File) ReadDir(count int) ([]fs.DirEntry, error) {
	v, err := jsutil.AwaitErr(f.Call("readdir", count))
	if err != nil {
		return nil, convertErr(err)
	}
	var entries []fs.DirEntry
	for i := 0; i < v.Length(); i++ {
		entries = append(entries, FileInfo{Value: v.Index(i)})
	}
	return entries, nil
}

type FileInfo struct {
	js.Value
}

var _ fs.FileInfo = FileInfo{}
var _ fs.DirEntry = FileInfo{}

func (fi FileInfo) Name() string {
	return fi.Get("name").String()
}

func (fi FileInfo) Mode() fs.FileMode {
	return pstat.UnixModeToFileMode(uint32(fi.Get("mode").Int()))
}

func (fi FileInfo) ModTime() time.Time {
	return time.Unix(int64(fi.Get("mtime").Int()), 0)
}

func (fi FileInfo) Size() int64 {
	return int64(fi.Get("size").Int())
}

func (fi FileInfo) IsDir() bool {
	return fi.Mode()&fs.ModeDir != 0
}

func (fi FileInfo) Sys() any {
	return nil
}

func (fi FileInfo) Type() fs.FileMode {
	return fi.Mode().Type()
}

func (fi FileInfo) Info() (fs.FileInfo, error) {
	return fi, nil
}
