//go:build js && wasm

package jsfs

import (
	"strconv"
	"strings"
	"syscall/js"

	"tractor.dev/wanix/fs"
)

func reflectHas(parent js.Value, key string) bool {
	return js.Global().Get("Reflect").Call("has", parent, js.ValueOf(key)).Bool()
}

func reflectDeleteProp(obj js.Value, key string) {
	js.Global().Get("Reflect").Call("deleteProperty", obj, js.ValueOf(key))
}

func (f *FS) Remove(name string) error {
	if strings.Contains(name, ":") {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
	}
	if name == "." {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
	}
	parts, err := splitPathParts(name)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
	}

	parent := f.root
	for i := 0; i < len(parts)-1; i++ {
		n := parent.Get(parts[i])
		if isNullish(n) {
			return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
		}
		parent = n
		if !canHaveProperties(parent) {
			return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
		}
	}
	key := parts[len(parts)-1]
	if !reflectHas(parent, key) {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}
	cur := parent.Get(key)

	if cur.IsUndefined() {
		if arrayIsArray(parent) {
			idx, convErr := strconv.Atoi(key)
			if convErr != nil {
				return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
			}
			parent.Call("splice", js.ValueOf(idx), js.ValueOf(1))
			return nil
		}
		reflectDeleteProp(parent, key)
		return nil
	}

	if err := reflectSet(parent, key, js.Undefined()); err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}
	return nil
}
