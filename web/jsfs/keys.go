//go:build js && wasm

package jsfs

import (
	"sort"
	"syscall/js"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

// listDirEntries returns children for an object directory (Object.keys).
func listDirEntries(v js.Value) []fs.DirEntry {
	keys := js.Global().Get("Object").Call("keys", v)
	entries := make([]fs.DirEntry, 0, keys.Length())
	for i := 0; i < keys.Length(); i++ {
		name := keys.Index(i).String()
		child := v.Get(name)
		mode := entryModeFor(child)
		entries = append(entries, fskit.Entry(name, mode))
	}
	return entries
}

// listObjViewEntries returns prototype-inclusive string keys (non-enumerable
// included). Symbol keys are omitted (path segments are strings only).
func listObjViewEntries(v js.Value) []fs.DirEntry {
	seen := map[string]struct{}{}
	var names []string

	cur := v
	for cur.Type() == js.TypeObject && !isNullish(cur) {
		ow := js.Global().Get("Reflect").Call("ownKeys", cur)
		for i := 0; i < ow.Length(); i++ {
			k := ow.Index(i)
			if k.Type() != js.TypeString {
				continue
			}
			s := k.String()
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			names = append(names, s)
		}
		cur = js.Global().Get("Object").Call("getPrototypeOf", cur)
	}

	sort.Strings(names)
	entries := make([]fs.DirEntry, 0, len(names))
	for _, name := range names {
		child := v.Get(name)
		mode := entryModeFor(child)
		entries = append(entries, fskit.Entry(name, mode))
	}
	return entries
}
