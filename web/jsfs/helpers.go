//go:build js && wasm

package jsfs

import (
	"strings"
	"sync"
	"syscall"
	"syscall/js"

	"tractor.dev/wanix/fs"
)

var typeofFn = sync.OnceValue(func() js.Value {
	return js.Global().Call("eval", js.ValueOf("(function(x){return typeof x})"))
})

func jsTypeof(v js.Value) string {
	return typeofFn().Invoke(v).String()
}

type viewSuffix int

const (
	suffixNone viewSuffix = iota
	suffixObj
	suffixRef
	suffixJSON
	suffixType
)

func parseSuffixSegment(seg string) (stem string, sfx viewSuffix) {
	// Longest match first for clarity (no overlap in spec).
	suffixes := []struct {
		s string
		v viewSuffix
	}{
		{":json", suffixJSON},
		{":type", suffixType},
		{":obj", suffixObj},
		{":ref", suffixRef},
	}
	for _, e := range suffixes {
		if strings.HasSuffix(seg, e.s) {
			return strings.TrimSuffix(seg, e.s), e.v
		}
	}
	return seg, suffixNone
}

// splitPathParts splits a fs path into non-empty segments (no trailing dot).
func splitPathParts(name string) ([]string, error) {
	if name == "." {
		return nil, nil
	}
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	return strings.Split(name, "/"), nil
}

func isNullish(v js.Value) bool {
	return v.IsNull() || v.IsUndefined()
}

func isCallable(v js.Value) bool {
	return v.Type() == js.TypeFunction
}

func arrayIsArray(v js.Value) bool {
	if v.Type() != js.TypeObject {
		return false
	}
	fn := js.Global().Get("Array").Get("isArray")
	return fn.Invoke(v).Bool()
}

// isDirectoryNode reports whether value should be listed as a directory in the
// default (non–:obj) view.
func isDirectoryNode(v js.Value) bool {
	if isNullish(v) {
		return false
	}
	if v.Type() == js.TypeFunction {
		return false
	}
	return v.Type() == js.TypeObject
}

func entryModeFor(child js.Value) fs.FileMode {
	if isDirectoryNode(child) {
		return fs.ModeDir | 0555
	}
	return 0644
}

// boxObject returns Object(primitiveOrObject), used for forced :obj traversal.
func boxObject(v js.Value) js.Value {
	return js.Global().Get("Object").Invoke(v)
}

func reflectApply(fn, this js.Value, args []js.Value) (out js.Value, err error) {
	arr := js.Global().Get("Array").New(len(args))
	for i := range args {
		arr.SetIndex(i, args[i])
	}
	ref := js.Global().Get("Reflect")
	defer func() {
		if r := recover(); r != nil {
			if fn.Type() == js.TypeFunction {
				if je, ok := r.(js.Error); ok {
					fn.Set("lastError", je.Value)
				} else {
					fn.Set("lastError", js.ValueOf(r))
				}
			}
			err = syscall.EIO
			out = js.Undefined()
		}
	}()
	out = ref.Call("apply", fn, this, arr)
	return out, nil
}

func resolveGlobalPath(path string) (js.Value, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return js.Null(), nil
	}
	if !strings.HasPrefix(path, "@") {
		return js.Undefined(), syscall.EINVAL
	}
	path = strings.TrimPrefix(path, "@")
	path = strings.Trim(path, "/")
	if path == "" {
		return js.Global(), nil
	}
	parts := strings.Split(path, "/")
	cur := js.Global()
	for _, p := range parts {
		if p == "" {
			continue
		}
		next := cur.Get(p)
		if isNullish(next) {
			return js.Undefined(), fs.ErrNotExist
		}
		cur = next
	}
	return cur, nil
}

func resolveAtArg(s string) (js.Value, error) {
	return resolveGlobalPath(s)
}

// jsValueString is JavaScript's String(value). syscall/js forbids Call on primitives
// like boolean — Value.Call("toString") panics there.
func jsValueString(v js.Value) string {
	return js.Global().Get("String").Invoke(v).String()
}

func jsToStringLine(v js.Value) []byte {
	return append([]byte(jsValueString(v)), '\n')
}

// reflectSet uses Reflect.set so failures surface as Go errors instead of
// WASM runtime panics from Value.Set on non-writable receivers.
func reflectSet(target js.Value, prop string, val js.Value) (err error) {
	defer func() {
		if recover() != nil {
			err = syscall.EINVAL
		}
	}()
	ok := js.Global().Get("Reflect").Call("set", target, js.ValueOf(prop), val).Bool()
	if !ok {
		return syscall.EACCES
	}
	return nil
}

// isBoxedPrimitiveObject distinguishes new String(...) etc. from plain objects.
func isBoxedPrimitiveObject(v js.Value) bool {
	if v.Type() != js.TypeObject || v.IsNull() {
		return false
	}
	return func() (ok bool) {
		defer func() {
			if recover() != nil {
				ok = false
			}
		}()
		g := js.Global()
		for _, name := range []string{"String", "Number", "Boolean", "Symbol", "BigInt"} {
			t := g.Get(name)
			if t.IsUndefined() || t.IsNull() || t.Type() != js.TypeFunction {
				continue
			}
			if v.InstanceOf(t) {
				return true
			}
		}
		return false
	}()
}

// truncateWriteBlocks reports values that cannot be replaced by POSIX-style
// open(O_TRUNC): plain objects and arrays (but not boxed primitives).
func truncateWriteBlocks(v js.Value) bool {
	if isCallable(v) || isNullish(v) {
		return false
	}
	if !isDirectoryNode(v) {
		return false
	}
	return !isBoxedPrimitiveObject(v)
}
