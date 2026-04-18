package wanix

import (
	"io/fs"

	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/internal"
	"tractor.dev/wanix/vnd"
)

var Version string

type Resource interface {
	fs.FS
	ID() string
}

type Factory func(id, kind string) Resource

func NewRoot() (*Task, error) {
	t := NewTaskFS()
	root, err := t.Alloc("", nil)
	if err != nil {
		return nil, err
	}

	wanixfs := fskit.MapFS{
		"version": fskit.RawNode([]byte(Version+"\n"), 0555),
		"vnd":     vnd.Assets,
	}
	if err := root.Namespace().Bind(wanixfs, ".", "#wanix"); err != nil {
		return nil, err
	}

	if err := root.Namespace().Bind(t, ".", "#task"); err != nil {
		return nil, err
	}

	return root, nil
}

var ControlFile = internal.ControlFile
var FieldFile = internal.FieldFile
