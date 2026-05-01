//go:build js && wasm

package jsfs

import (
	"syscall/js"

	"tractor.dev/wanix/fs"
)

// FS projects a rooted JavaScript value graph as an fs.FS (see DESIGN.md).
type FS struct {
	root js.Value
}

func NewFS(v js.Value) *FS {
	return &FS{root: v}
}

func (f *FS) Value() js.Value {
	return f.root
}

func thisForCall(loc resolved) js.Value {
	if !loc.hasParent {
		return js.Undefined()
	}
	if canHaveProperties(loc.parent) {
		return loc.parent
	}
	return js.Undefined()
}

func liveAt(loc resolved, root js.Value) func() js.Value {
	return func() js.Value {
		if !loc.hasParent {
			return root
		}
		return loc.parent.Get(loc.key)
	}
}

func (f *FS) Open(name string) (fs.File, error) {
	parts, err := splitPathParts(name)
	if err != nil {
		return nil, err
	}
	loc, err := walkPath(f.root, parts)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	switch loc.sfx {
	case suffixObj:
		return newDir(name, loc.target, listObjView), nil
	case suffixRef:
		return newRefFile(name, loc), nil
	case suffixType:
		return newTypeFile(name, loc.target), nil
	case suffixJSON:
		if isCallable(loc.target) {
			return newFuncFile(name, loc.target, thisForCall(loc), true), nil
		}
		return newJSONValueFile(name, loc, liveAt(loc, f.root)), nil
	default:
		if isDirectoryNode(loc.target) {
			return newDir(name, loc.target, listObjectKeys), nil
		}
		if isCallable(loc.target) {
			return newFuncFile(name, loc.target, thisForCall(loc), false), nil
		}
		return newPrimitiveFile(name, liveAt(loc, f.root), loc.parent, loc.key, loc.hasParent), nil
	}
}

var _ fs.FS = (*FS)(nil)
var _ fs.RemoveFS = (*FS)(nil)
