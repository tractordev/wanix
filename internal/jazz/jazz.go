package jazz

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"github.com/spf13/afero"
)

// KEEP IN MIND this whole thing is full of shortcuts for the moment
// ALSO this needs to be rethought in the new architecture, so it is currently broken

func jazzCall(name string, args ...any) js.Value {
	v := js.Global().Get("wanix").Get("jazzfs").Get(name).Invoke(args...)
	if js.Global().Get("wanix").Get("jazzfs").Get("isPromise").Invoke(v).Bool() {
		done := make(chan js.Value, 1)
		v.Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
			done <- args[0]
			return nil
		}))
		return <-done
	} else {
		return v
	}
}

type jazzfs struct {
}

func NewJazzFs() afero.Fs {
	return &jazzfs{}
}

func (fs *jazzfs) walkTo(path string) (js.Value, error) {
	root := js.Global().Get("wanix").Get("jazzfs").Get("root").Invoke()
	ret := jazzCall("walkTo", root, path)
	if ret.IsUndefined() || ret.IsNull() {
		return js.Undefined(), os.ErrNotExist
	}
	return ret, nil
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (fs *jazzfs) Create(name string) (afero.File, error) {
	return fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (fs *jazzfs) Mkdir(name string, perm os.FileMode) error {
	return fs.MkdirAll(name, perm)
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (fs *jazzfs) MkdirAll(path string, perm os.FileMode) error {
	v, _ := fs.walkTo(path)
	if !v.IsUndefined() {
		return nil
	}
	var parts []string
	for _, part := range strings.Split(path, "/") {
		parts = append(parts, part)
		p := strings.Join(parts, "/")
		v, _ := fs.walkTo(p)
		if !v.IsUndefined() {
			continue
		}
		jazzCall("mkdir", p)
	}
	return nil
}

// Open opens a file, returning it or an error, if any happens.
func (fs *jazzfs) Open(name string) (afero.File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile opens a file using the given flags and the given mode.
func (fs *jazzfs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	// log("jazz open", name)
	v, err := fs.walkTo(name)
	if err != nil {
		if err == os.ErrNotExist && flag&os.O_CREATE > 0 {
			if strings.Contains(name, "/") {
				fs.MkdirAll(filepath.Dir(name), 0755)
			}
			log("jazz makenode", name)
			jazzCall("makeNode", name)
			v, err = fs.walkTo(name)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	f := &jazzfile{node: v, path: name}
	if flag&os.O_APPEND > 0 {
		f.Seek(0, 2)
	}
	if flag&os.O_TRUNC > 0 {
		f.Seek(0, 0)
	}
	return f, nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (fs *jazzfs) Remove(name string) error {
	_, err := fs.walkTo(name)
	if err != nil {
		return err
	}
	jazzCall("remove", name)
	return nil
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (fs *jazzfs) RemoveAll(path string) error {
	// TODO: this does not remove children
	jazzCall("remove", path)
	return nil
}

// Rename renames a file.
func (fs *jazzfs) Rename(oldname string, newname string) error {
	jazzCall("rename", oldname, newname)
	return nil
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (fs *jazzfs) Stat(name string) (os.FileInfo, error) {
	//log("jazz stat", name)
	v, err := fs.walkTo(name)
	if err != nil {
		return nil, err
	}
	d := v.Get("coMap").Call("get", "dataID")
	size := 0
	isDir := true
	modTime := 0
	if !d.IsUndefined() {
		isDir = false
		ds := v.Get("coMap").Call("get", "dataSize")
		if !ds.IsUndefined() {
			size = ds.Int()
		}
		modTime = v.Get("edits").Get("dataID").Get("at").Call("getTime").Int() / 1000
	}
	return &jazzinfo{
		name:    filepath.Base(name),
		size:    int64(size),
		isDir:   isDir,
		modTime: int64(modTime),
	}, nil
}

// The name of this FileSystem
func (fs *jazzfs) Name() string {
	return "Jazz"
}

// Chmod changes the mode of the named file to mode.
func (fs *jazzfs) Chmod(name string, mode os.FileMode) error {
	log("not implemented") // TODO: Implement
	return nil
}

// Chown changes the uid and gid of the named file.
func (fs *jazzfs) Chown(name string, uid int, gid int) error {
	log("not implemented") // TODO: Implement
	return nil
}

// Chtimes changes the access and modification times of the named file
func (fs *jazzfs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	log("not implemented") // TODO: Implement
	return nil
}

type jazzfile struct {
	node   js.Value
	path   string
	offset int64
	rcache []byte
	lastId string
	dirty  bool
	wcache []byte
	wmu    sync.Mutex
}

func (f *jazzfile) data() ([]byte, error) {
	d := f.node.Get("coMap").Call("get", "dataID")
	if d.IsUndefined() {
		log("empty file", f.path)
		return []byte{}, nil
	}
	if d.String() == f.lastId {
		return f.rcache, nil
	}
	// ch := make(chan []byte)
	jsbuf := jazzCall("fetchFile", d)
	buf := make([]byte, jsbuf.Length())
	js.CopyBytesToGo(buf, jsbuf)
	// ch <- buf
	// select {
	// case <-time.After(2 * time.Second):
	// 	log("slow file load, returning cached", f.path)
	// 	return f.rcache, nil
	// case b := <-ch:
	f.rcache = buf
	f.lastId = d.String()
	return buf, nil
	// }
}

func (f *jazzfile) setData(data []byte) {
	log("jazz flush")
	f.wmu.Lock()
	defer f.wmu.Unlock()
	buf := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(buf, data)
	ret := jazzCall("makeFile", buf)
	f.node.Call("mutate", js.FuncOf(func(this js.Value, args []js.Value) any {
		args[0].Call("set", "dataID", ret.Get("id"))
		args[0].Call("set", "dataSize", len(data))
		return nil
	}))
	log("jazz mutated", ret.Get("id"))
}

func (f *jazzfile) Close() error {
	f.offset = 0
	if f.dirty {
		return f.Sync()
	}
	return nil
}

func (f *jazzfile) Read(p []byte) (n int, err error) {
	d, err := f.data()
	if err != nil {
		return 0, err
	}
	if f.offset >= int64(len(d)) {
		return 0, io.EOF
	}
	rest := d[f.offset:]
	if len(rest) < len(p) {
		n = len(rest)
	} else {
		n = len(p)
	}
	copy(p, rest[:n])
	f.offset += int64(n)
	return n, nil
}

func (f *jazzfile) ReadAt(p []byte, off int64) (n int, err error) {
	log("not implemented") // TODO: Implement
	return 0, nil
}

func (f *jazzfile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0: // file start
		f.offset = offset
	case 1: // current offset
		f.offset += offset
	case 2: // file end
		d, err := f.data()
		if err != nil {
			return 0, err
		}
		f.offset = int64(len(d)) + offset
	}
	return f.offset, nil
}

func (f *jazzfile) Write(p []byte) (n int, err error) {
	cur, err := f.data()
	if err != nil {
		return 0, err
	}
	new := make([]byte, f.offset+int64(len(p)))
	copy(new, cur[:f.offset])
	copy(new[f.offset:], p)
	f.wcache = new
	f.offset += int64(len(p))
	f.dirty = true
	return len(p), nil
}

func (f *jazzfile) WriteAt(p []byte, off int64) (n int, err error) {
	log("not implemented") // TODO: Implement
	return 0, nil
}

func (f *jazzfile) Name() string {
	return filepath.Base(f.path)
}

func (f *jazzfile) Readdir(count int) (fis []os.FileInfo, err error) {
	// TODO: use count
	names := js.Global().Get("Object").Call("keys", f.node)
	for i := 0; i < names.Length(); i++ {
		name := names.Index(i).String()
		node := f.node.Get(name)
		file := &jazzfile{node: node, path: filepath.Join(f.path, name)}
		fi, _ := file.Stat()
		fis = append(fis, fi)
	}
	return
}

func (f *jazzfile) Readdirnames(n int) (names []string, err error) {
	var fi []os.FileInfo
	fi, err = f.Readdir(-1)
	if err != nil {
		return
	}
	for _, f := range fi {
		names = append(names, f.Name())
	}
	return
}

func (f *jazzfile) Stat() (os.FileInfo, error) {
	d := f.node.Get("coMap").Call("get", "dataID")
	size := 0
	isDir := true
	modTime := 0
	if !d.IsUndefined() {
		isDir = false
		ds := f.node.Get("coMap").Call("get", "dataSize")
		if !ds.IsUndefined() {
			size = ds.Int()
		}
		modTime = f.node.Get("edits").Get("dataID").Get("at").Call("getTime").Int() / 1000
	}
	return &jazzinfo{
		name:    filepath.Base(f.path),
		size:    int64(size),
		isDir:   isDir,
		modTime: int64(modTime),
	}, nil
}

func (f *jazzfile) Sync() error {
	if !f.dirty {
		return nil
	}
	f.dirty = false
	ch := make(chan bool, 1)
	go func() {
		f.setData(f.wcache)
		ch <- true
	}()
	select {
	case <-time.After(4 * time.Second):
		log("long write, continuing async!")
		return nil
	case <-ch:
		return nil
	}
}

func (f *jazzfile) Truncate(size int64) error {
	log("not implemented") // TODO: Implement
	return nil
}

func (f *jazzfile) WriteString(s string) (ret int, err error) {
	log("not implemented") // TODO: Implement
	return 0, nil
}

type jazzinfo struct {
	name    string
	size    int64
	isDir   bool
	modTime int64
}

func (i *jazzinfo) Name() string {
	return i.name
}

func (i *jazzinfo) Size() int64 {
	return i.size
}

func (i *jazzinfo) Mode() fs.FileMode {
	if i.isDir {
		return 0755 | fs.ModeDir
	}
	return 0644
}

func (i *jazzinfo) ModTime() time.Time {
	return time.Unix(i.modTime, 0)
}

func (i *jazzinfo) IsDir() bool {
	return i.isDir
}

func (i *jazzinfo) Sys() any {
	return nil
}
