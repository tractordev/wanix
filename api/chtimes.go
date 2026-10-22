package api

import (
	"time"

	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/wanix/fs"
)

// asFloat64 coerces a CBOR-decoded number to float64. CBOR encodes
// integer-valued floats (e.g. whole-second timestamps) as integers, so a
// plain assertion to float64 is not safe.
func asFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}

func (s *syscaller) chtimes(r rpc.Responder, c *rpc.Call) {
	var args []any
	c.Receive(&args)

	path, ok := args[0].(string)
	if !ok {
		panic("arg 0 is not a string")
	}

	// atime and mtime are in seconds (with fractional parts)
	atimeSec, ok := asFloat64(args[1])
	if !ok {
		panic("arg 1 is not numeric")
	}
	atime := time.Unix(int64(atimeSec), int64((atimeSec-float64(int64(atimeSec)))*1e9))

	mtimeSec, ok := asFloat64(args[2])
	if !ok {
		panic("arg 2 is not numeric")
	}
	mtime := time.Unix(int64(mtimeSec), int64((mtimeSec-float64(int64(mtimeSec)))*1e9))

	err := fs.Chtimes(s.task.NS(), path, atime, mtime)
	if err != nil {
		r.Return(err)
		return
	}
}
