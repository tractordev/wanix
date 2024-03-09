package jazz

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"syscall/js"
	"time"

	"tractor.dev/toolkit-go/engine/fs/watchfs"
	"tractor.dev/wanix/internal/cache"
)

func fsUtil(name string, args ...any) js.Value {
	v := js.Global().Get("jazz").Get("fsutil").Get(name).Invoke(args...)
	if js.Global().Get("jazz").Get("fsutil").Get("isPromise").Invoke(v).Bool() {
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
	cache *cache.C // speed up slow, frequent stat
}

func NewJazzFs() fs.FS {
	return &jazzfs{
		cache: cache.New(2 * time.Second),
	}
}

func (fs *jazzfs) Create(name string) (fs.File, error) {
	return fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

func (fs *jazzfs) Open(name string) (fs.File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0)
}

func (fs *jazzfs) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	fmt.Println("open:", name)
	v := fsUtil("walk", name)
	if v.IsNull() {
		if flag&os.O_CREATE == 0 {
			return nil, os.ErrNotExist
		}
		// if strings.Contains(name, "/") {
		// 	fs.MkdirAll(filepath.Dir(name), 0755)
		// }
		fsUtil("writeFile", name, "")
		v = fsUtil("walk", name)
		if v.IsNull() {
			fmt.Println("open: unable to find created file")
			return nil, os.ErrNotExist
		}
	}
	f := &jazzfile{node: v, path: name, fs: fs}
	if flag&os.O_APPEND > 0 {
		f.Seek(0, 2)
	}
	if flag&os.O_TRUNC > 0 {
		f.Seek(0, 0)
	}
	return f, nil
}

func (fs *jazzfs) Mkdir(name string, perm fs.FileMode) error {
	ret := fsUtil("mkdir", name)
	if ret.IsNull() {
		return os.ErrNotExist
	}
	return nil
}

func (fs *jazzfs) MkdirAll(path string, perm fs.FileMode) error {
	fsUtil("mkdirAll", path)
	return nil
}

// should fail if file does not exist
func (fs *jazzfs) Remove(name string) error {
	ret := fsUtil("remove", name)
	if ret.IsNull() {
		return os.ErrNotExist
	}
	return nil
}

// does not fail if the path does not exist (return nil).
func (fs *jazzfs) RemoveAll(path string) error {
	// TODO: this does not remove children
	fsUtil("remove", path)
	return nil
}

func (fs *jazzfs) Rename(oldname string, newname string) error {
	ret := fsUtil("rename", oldname, newname)
	if ret.IsNull() {
		return os.ErrNotExist
	}
	return nil
}

func (fs *jazzfs) Stat(name string) (fs.FileInfo, error) {
	v, found := fs.cache.Get(name)
	if found {
		fmt.Println("stat cached:", name)
		return v.(*jazzinfo), nil
	}
	fmt.Println("stat:", name)
	ret := fsUtil("stat", name)
	if ret.IsNull() {
		return nil, os.ErrNotExist
	}
	vv := &jazzinfo{ret}
	fs.cache.Set(name, vv, cache.DefaultExpiration)
	return vv, nil
}

func (fs *jazzfs) ReadDir(name string) (entries []fs.DirEntry, err error) {
	fmt.Println("readdir:", name)
	ret := fsUtil("readdir", name)
	if ret.IsNull() {
		return nil, os.ErrNotExist
	}
	for i := 0; i < ret.Length(); i++ {
		fi, err := fs.Stat(filepath.Join(name, ret.Index(i).String()))
		if err != nil {
			fmt.Println("stat err:", err, ret.Index(i).String())
			return nil, err
		}
		entries = append(entries, fi.(*jazzinfo))
	}
	return
}

func (fs *jazzfs) Chmod(name string, mode os.FileMode) error {
	fmt.Println("chmod: not implemented") // TODO: Implement
	return nil
}

func (fs *jazzfs) Chown(name string, uid int, gid int) error {
	fmt.Println("chown: not implemented") // TODO: Implement
	return nil
}

func (fs *jazzfs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	fmt.Println("chtimes: not implemented") // TODO: Implement
	return nil
}

func (fs *jazzfs) Watch(name string, cfg *watchfs.Config) (*watchfs.Watch, error) {
	fmt.Println("watch:", name)
	if cfg == nil {
		cfg = &watchfs.Config{}
	}
	watch, inbox, closer := watchfs.NewWatch(name, *cfg)
	if cfg.Handler != nil {
		go func() {
			for e := range watch.Iter() {
				cfg.Handler(e)
			}
		}()
	}
	inflateEvent := func(v js.Value) watchfs.Event {
		return watchfs.Event{
			Path:     v.Get("path").String(),
			OldPath:  v.Get("path").String(),
			Type:     watchfs.EventWrite,
			Err:      nil,
			FileInfo: &jazzinfo{v},
		}
	}
	go func() {
		defer close(inbox)
		token := fsUtil("watch", name, js.FuncOf(func(this js.Value, args []js.Value) any {
			select {
			case inbox <- inflateEvent(args[0].Get("detail")):
			default:
			}
			return nil
		}))
		<-closer
		fsUtil("unwatch", token)
	}()
	return watch, nil
}

type jazzfile struct {
	fs      *jazzfs
	node    js.Value
	path    string
	offset  int64
	cache   []byte
	lastFid string
	dirty   bool
	wmu     sync.Mutex
}

func (f *jazzfile) Name() string {
	return filepath.Base(f.path)
}

func (f *jazzfile) Stat() (fs.FileInfo, error) {
	return f.fs.Stat(f.path)
}

func (f *jazzfile) ReadDir(n int) ([]fs.DirEntry, error) {
	return f.fs.ReadDir(f.path)
}

func (f *jazzfile) data() ([]byte, error) {
	fi, err := f.fs.Stat(f.path)
	if err != nil {
		return nil, err
	}
	stat := fi.(*jazzinfo)
	fid := stat.val.Get("fileID")
	if fid.IsUndefined() {
		return []byte{}, os.ErrNotExist
	}
	if fid.String() == f.lastFid {
		return f.cache, nil
	}
	jsbuf := fsUtil("fetchFile", fid)
	buf := make([]byte, jsbuf.Length())
	js.CopyBytesToGo(buf, jsbuf)
	// ch <- buf
	// select {
	// case <-time.After(2 * time.Second):
	// 	log("slow file load, returning cached", f.path)
	// 	return f.rcache, nil
	// case b := <-ch:
	f.cache = buf
	f.lastFid = fid.String()
	return buf, nil
	// }
}

func (f *jazzfile) setData(data []byte) {
	f.wmu.Lock()
	defer f.wmu.Unlock()
	buf := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(buf, data)
	ret := fsUtil("writeFile", f.path, buf)
	if ret.IsNull() {
		fmt.Println("writeFile failed:", f.path)
		return
	}
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
	n = copy(new[f.offset:], p)
	f.cache = new
	f.offset += int64(n)
	f.dirty = true
	return n, nil
}

func (f *jazzfile) Close() error {
	f.offset = 0
	if f.dirty {
		return f.Sync()
	}
	return nil
}

func (f *jazzfile) Sync() error {
	if !f.dirty {
		return nil
	}
	f.dirty = false
	ch := make(chan bool, 1)
	go func() {
		f.setData(f.cache)
		ch <- true
	}()
	select {
	case <-time.After(4 * time.Second):
		fmt.Println("long write, continuing async!")
		return nil
	case <-ch:
		return nil
	}
}

// func (f *jazzfile) WriteAt(p []byte, off int64) (n int, err error) {
// 	fmt.Println("writeAt: not implemented") // TODO: Implement
// 	return 0, nil
// }

// func (f *jazzfile) ReadAt(p []byte, off int64) (n int, err error) {
// 	fmt.Println("readAt: not implemented") // TODO: Implement
// 	return 0, nil
// }

// func (f *jazzfile) Truncate(size int64) error {
// 	fmt.Println("truncate: not implemented") // TODO: Implement
// 	return nil
// }

type jazzinfo struct {
	val js.Value
}

func (i *jazzinfo) Name() string               { return i.val.Get("name").String() }
func (i *jazzinfo) Size() int64                { return int64(i.val.Get("size").Int()) }
func (i *jazzinfo) ModTime() time.Time         { return time.Unix(int64(i.val.Get("mtime").Int()), 0) }
func (i *jazzinfo) IsDir() bool                { return i.val.Get("isDir").Bool() }
func (i *jazzinfo) Sys() any                   { return nil }
func (i *jazzinfo) Info() (fs.FileInfo, error) { return i, nil }
func (i *jazzinfo) Type() fs.FileMode          { return i.Mode() }

func (i *jazzinfo) Mode() fs.FileMode {
	if i.IsDir() {
		return 0755 | fs.ModeDir
	}
	return 0644
}
