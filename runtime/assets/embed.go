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
// If preferTiny is true, it will prefer the tinygo build.
// If no wasm file is found, it will return an error.
func WasmFS(preferTiny bool) (fs.FS, error) {
	wasmFsys := fskit.MapFS{}
	if ok, _ := fs.Exists(Dir, "wanix.go.wasm"); ok {
		wasmFsys["wanix.wasm"], _ = fs.Sub(Dir, "wanix.go.wasm")
	}
	if len(wasmFsys) > 0 && !preferTiny {
		return wasmFsys, nil
	}
	if ok, _ := fs.Exists(Dir, "wanix.tinygo.wasm"); ok {
		wasmFsys["wanix.wasm"], _ = fs.Sub(Dir, "wanix.tinygo.wasm")
	}
	if len(wasmFsys) > 0 {
		return wasmFsys, nil
	}
	return nil, errors.New("no wanix wasm found in assets")
}
