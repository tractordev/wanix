//go:build js && wasm

// File System Access API
package fsa

import (
	"cmp"
	"encoding/json"
	"errors"
	"log"
	"syscall/js"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/web/jsutil"
)

// user selected directory
func ShowDirectoryPicker() fs.FS {
	dir := jsutil.Await(js.Global().Get("window").Call("showDirectoryPicker"))
	return FS(dir)
}

// origin private file system
func OPFS() (fs.FS, error) {
	dir, err := jsutil.AwaitErr(js.Global().Get("navigator").Get("storage").Call("getDirectory"))
	if err != nil {
		return nil, err
	}
	fsys := FS(dir)
	go func() {
		b, err := fs.ReadFile(fsys, "#stat")
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return
			}
			log.Println("fsa: opfs: read #stat:", err)
			return
		}
		statCache.Clear()
		var stats []Stat
		if err := json.Unmarshal(b, &stats); err != nil {
			log.Println("fsa: opfs: unmarshal #stat:", err)
			return
		}
		for _, s := range stats {
			statCache.Store(s.Name, s)
		}
	}()
	return fsys, nil
}

func DirHandleFile(fsys FS, name string, v js.Value) fs.File {
	var entries []fs.DirEntry
	err := jsutil.AsyncIter(v.Call("values"), func(e js.Value) error {
		var mode fs.FileMode
		var size int64
		name := e.Get("name").String()

		v, cached := statCache.Load(name)
		if cached {
			mode = v.(Stat).Mode
		}

		isDir := e.Get("kind").String() == "directory"
		if isDir {
			if mode&0777 == 0 {
				mode |= DefaultDirMode
			}
			mode |= fs.ModeDir
		} else {
			if mode&0777 == 0 {
				mode |= DefaultFileMode
			}
			size = int64(jsutil.Await(e.Call("getFile")).Get("size").Int())
		}
		entries = append(entries, fskit.Entry(name, mode, size))
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	fname := cmp.Or(v.Get("name").String(), ".")
	return fskit.DirFile(fskit.Entry(fname, 0755|fs.ModeDir), entries...)
}
