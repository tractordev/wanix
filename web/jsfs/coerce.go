//go:build js && wasm

package jsfs

import (
	"strings"
	"syscall/js"

	"tractor.dev/wanix/fs"
)

func trimEndGo(s string) string {
	return strings.TrimRight(s, " \t\r\n\v\f\u0085\u00a0")
}

func parseBoolString(s string) (bool, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "0", "false", "no", "n", "off":
		return false, true
	case "1", "true", "yes", "y", "on":
		return true, true
	default:
		return false, false
	}
}

func coercePrimitive(existing js.Value, trimmed string) (js.Value, error) {
	switch existing.Type() {
	case js.TypeString:
		return js.ValueOf(trimmed), nil
	case js.TypeNumber:
		return js.Global().Get("Number").Invoke(trimmed), nil
	case js.TypeBoolean:
		if b, ok := parseBoolString(trimmed); ok {
			return js.ValueOf(b), nil
		}
		return js.Undefined(), &fs.PathError{Op: "write", Err: fs.ErrInvalid}
	case js.TypeSymbol:
		return js.Global().Get("Symbol").Get("for").Invoke(trimmed), nil
	default:
		if jsTypeof(existing) == "bigint" {
			return tryBigInt(trimmed)
		}
		return js.ValueOf(trimmed), nil
	}
}

func tryBigInt(s string) (bi js.Value, err error) {
	defer func() {
		if recover() != nil {
			err = fs.ErrInvalid
		}
	}()
	bi = js.Global().Get("BigInt").Invoke(s)
	return bi, err
}
