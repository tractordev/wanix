package assets

import (
	"embed"
	"errors"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

//go:embed *
var Dir embed.FS

// WasmFS returns a filesystem containing the wanix.wasm file.
// If preferDebug is true, it will prefer the Go build.
// If no wasm file is found, it will return an error.
func WasmFS(preferDebug bool) (fs.FS, error) {
	wasmFsys := fskit.MapFS{}
	if ok, _ := fs.Exists(Dir, "wanix.debug.wasm"); ok {
		wasmFsys["wanix.wasm"], _ = fs.Sub(Dir, "wanix.debug.wasm")
	}
	if len(wasmFsys) > 0 && preferDebug {
		return wasmFsys, nil
	}
	if ok, _ := fs.Exists(Dir, "wanix.wasm"); ok {
		wasmFsys["wanix.wasm"], _ = fs.Sub(Dir, "wanix.wasm")
	}
	if len(wasmFsys) > 0 {
		return wasmFsys, nil
	}
	return nil, errors.New("no wanix wasm found in assets")
}
