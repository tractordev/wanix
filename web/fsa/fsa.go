//go:build js && wasm

// File System Access API
package fsa

import (
	"strings"
	"syscall/js"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/web/jsutil"
)

// user selected directory
func ShowDirectoryPicker() fs.FS {
	dir := jsutil.Await(js.Global().Get("window").Call("showDirectoryPicker"))
	return NewFS(dir)
}

// origin private file system
// OPFS returns a filesystem for the Origin Private File System.
// It can take optional path arguments to return a subdirectory:
//
//	OPFS() - returns root OPFS
//	OPFS("path/to/dir") - returns subdirectory at path
//	OPFS("path", "to", "dir") - returns subdirectory using path parts
func OPFS(pathParts ...string) (fs.FS, error) {
	rootDir, err := jsutil.AwaitErr(js.Global().Get("navigator").Get("storage").Call("getDirectory"))
	if err != nil {
		return nil, err
	}

	// Create the root FS instance
	rootFS := NewFS(rootDir)

	// Navigate to subdirectory if path parts provided
	targetDir := rootDir
	if len(pathParts) > 0 {
		// Join path parts and split by "/"
		var allParts []string
		for _, part := range pathParts {
			if part != "" {
				parts := strings.Split(strings.Trim(part, "/"), "/")
				for _, p := range parts {
					if p != "" {
						allParts = append(allParts, p)
					}
				}
			}
		}

		// Navigate to the target directory
		current := rootDir
		for _, part := range allParts {
			current, err = jsutil.AwaitErr(current.Call("getDirectoryHandle", part, map[string]any{"create": true}))
			if err != nil {
				return nil, err
			}
		}
		targetDir = current
	}

	// Create the target FS instance
	var fsys *FS
	if targetDir.Equal(rootDir) {
		// Target is root - use the root FS and set opfsRoot to itself
		fsys = rootFS
		fsys.opfsRoot = rootFS

		// Initialize global metadata store with this root
		if err := GetMetadataStore().Initialize(fsys); err != nil {
			return nil, err
		}
	} else {
		// Target is subdirectory - create new FS with root reference
		fsys = NewFS(targetDir)
		fsys.opfsRoot = rootFS

		// Initialize global metadata store with root
		if err := GetMetadataStore().Initialize(rootFS); err != nil {
			return nil, err
		}
	}

	return fsys, nil
}
