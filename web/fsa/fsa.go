//go:build js && wasm

// File System Access API
package fsa

import (
	"path"
	"sync"
	"syscall/js"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/web/jsutil"
)

// user selected directory
func ShowDirectoryPicker() fs.FS {
	dir := jsutil.Await(js.Global().Get("window").Call("showDirectoryPicker"))
	return NewFS(dir)
}

var opfsSingleton *FS
var opfsMu sync.Mutex

// origin private file system
// OPFS returns a filesystem for the Origin Private File System.
// It can take optional path arguments to return a subdirectory:
//
//	OPFS() - returns root OPFS
//	OPFS("path/to/dir") - returns subdirectory at path
//	OPFS("path", "to", "dir") - returns subdirectory using path parts
func OPFS(pathParts ...string) (fs.FS, error) {
	opfsMu.Lock()
	singleton := opfsSingleton
	opfsMu.Unlock()

	if singleton == nil {
		rootDir, err := jsutil.AwaitErr(js.Global().Get("navigator").Get("storage").Call("getDirectory"))
		if err != nil {
			return nil, err
		}
		fsys := NewFS(rootDir)
		if err := Metadata().Initialize(fsys); err != nil {
			return nil, err
		}
		opfsMu.Lock()
		opfsSingleton = fsys
		opfsMu.Unlock()
	}

	if len(pathParts) == 0 {
		return opfsSingleton, nil
	}

	return fs.Sub(opfsSingleton, path.Join(pathParts...))
}
