//go:build js && wasm

package jsfs

import (
	"syscall/js"

	"tractor.dev/wanix/fs"
)

type resolved struct {
	target    js.Value
	sfx       viewSuffix
	parent    js.Value
	hasParent bool
	key       string // property name on parent for last hop stem
}

func canHaveProperties(v js.Value) bool {
	if isNullish(v) {
		return false
	}
	t := v.Type()
	return t == js.TypeObject || t == js.TypeFunction
}

func walkPath(root js.Value, parts []string) (r resolved, err error) {
	cur := root
	if len(parts) == 0 {
		r.target = cur
		r.sfx = suffixNone
		return r, nil
	}

	for i := 0; i < len(parts); i++ {
		stem, segSfx := parseSuffixSegment(parts[i])
		if stem == "" {
			return r, fs.ErrInvalid
		}
		last := i == len(parts)-1

		if !last {
			if segSfx != suffixNone && segSfx != suffixObj {
				return r, fs.ErrInvalid
			}
			if segSfx == suffixObj {
				if !reflectHas(cur, stem) {
					return r, fs.ErrNotExist
				}
				n := cur.Get(stem)
				if isNullish(n) {
					// Object(undefined|null) throws; cannot descend with :obj.
					return r, fs.ErrNotExist
				}
				cur = boxObject(n)
				continue
			}
			if !reflectHas(cur, stem) {
				return r, fs.ErrNotExist
			}
			n := cur.Get(stem)
			if isNullish(n) || !canHaveProperties(n) {
				return r, fs.ErrNotExist
			}
			cur = n
			continue
		}

		if !reflectHas(cur, stem) {
			return r, fs.ErrNotExist
		}
		n := cur.Get(stem)
		r.parent = cur
		r.hasParent = true
		r.key = stem
		switch segSfx {
		case suffixObj:
			if isNullish(n) {
				return r, fs.ErrNotExist
			}
			cur = boxObject(n)
			r.sfx = suffixObj
		case suffixRef, suffixJSON, suffixType:
			cur = n
			r.sfx = segSfx
		default:
			cur = n
			r.sfx = suffixNone
		}
	}

	r.target = cur
	return r, nil
}

// walkToParent returns the receiver of the last path segment for Set/OpenFile.
func walkToParent(root js.Value, parts []string) (parent js.Value, key string, err error) {
	if len(parts) < 1 {
		return js.Undefined(), "", fs.ErrInvalid
	}
	parent = root
	for i := 0; i < len(parts)-1; i++ {
		seg := parts[i]
		if !reflectHas(parent, seg) {
			err = fs.ErrNotExist
			return
		}
		n := parent.Get(seg)
		if isNullish(n) || !canHaveProperties(n) {
			err = fs.ErrNotExist
			return
		}
		parent = n
	}
	key = parts[len(parts)-1]
	return
}
