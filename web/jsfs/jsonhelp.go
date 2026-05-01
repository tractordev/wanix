//go:build js && wasm

package jsfs

import (
	"strings"
	"syscall"
	"syscall/js"

	"tractor.dev/wanix/fs"
)

func jsonParseArray(s string) (v js.Value, err error) {
	defer func() {
		if recover() != nil {
			err = fs.ErrInvalid
		}
	}()
	if strings.TrimSpace(s) == "" {
		return js.Undefined(), fs.ErrInvalid
	}
	v = js.Global().Get("JSON").Call("parse", js.ValueOf(s))
	if !js.Global().Get("Array").Get("isArray").Invoke(v).Bool() {
		return js.Undefined(), fs.ErrInvalid
	}
	return v, nil
}

func jsonArrayToArgs(arr js.Value) ([]js.Value, error) {
	n := arr.Get("length").Int()
	out := make([]js.Value, 0, n)
	for i := 0; i < n; i++ {
		el := arr.Index(i)
		if p, ok := atRefPathFromObject(el); ok {
			rv, err := resolveGlobalPathBare(p)
			if err != nil {
				return nil, err
			}
			out = append(out, rv)
			continue
		}
		out = append(out, el)
	}
	return out, nil
}

func atRefPathFromObject(v js.Value) (string, bool) {
	if v.Type() != js.TypeObject || v.IsNull() {
		return "", false
	}
	keys := js.Global().Get("Object").Call("keys", v)
	if keys.Length() != 1 {
		return "", false
	}
	if keys.Index(0).String() != "@" {
		return "", false
	}
	p := v.Get("@").String()
	return p, true
}

func resolveGlobalPathBare(p string) (js.Value, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return js.Undefined(), fs.ErrInvalid
	}
	if !strings.HasPrefix(p, "@") {
		p = "@" + p
	}
	return resolveGlobalPath(p)
}

func jsonStringifyLine(v js.Value) (b []byte, err error) {
	defer func() {
		if recover() != nil {
			err = syscall.EIO
		}
	}()
	s := js.Global().Get("JSON").Call("stringify", v).String()
	b = append([]byte(s), '\n')
	return b, nil
}
