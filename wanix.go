package wanix

import (
	"io/fs"

	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/misc"
)

var Version string

type Resource interface {
	fs.FS
	ID() string
}

type Factory func(id, kind string) Resource

func NewRoot() (*Task, error) {
	return NewRootWithTasks(NewTaskFS())
}

func NewRootWithTasks(t *TaskFS) (*Task, error) {
	root, err := t.Alloc("auto", nil)
	if err != nil {
		return nil, err
	}

	wanixfs := fskit.MapFS{
		"version": fskit.RawNode([]byte(Version+"\n"), 0644),
	}
	if err := root.NS().Bind(wanixfs, ".", "#wanix"); err != nil {
		return nil, err
	}

	if err := root.NS().Bind(t, ".", "#task"); err != nil {
		return nil, err
	}

	return root, nil
}

var ControlFile = misc.ControlFile
var FieldFile = misc.FieldFile
