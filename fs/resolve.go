package fs

import (
	"fmt"
	"log"
	"path"
)

type ResolveFS interface {
	FS
	Resolve(name string) (FS, string, error)
}

func ResolveAs[T FS](fsys FS, name string) (T, string, error) {
	var tfsys T

	rfsys, rname, err := Resolve(fsys, name)
	if err != nil {
		return tfsys, "", err
	}

	if v, ok := rfsys.(T); ok {
		tfsys = v
	} else {
		return tfsys, "", fmt.Errorf("resolve: %w on %T", ErrNotSupported, rfsys)
	}

	return tfsys, rname, nil
}

func Resolve(fsys FS, name string) (rfsys FS, rname string, err error) {
	if name == "." {
		// for now we'll limit using ResolveFS to roots
		if rfsys, ok := fsys.(ResolveFS); ok {
			log.Println("resolve:", name)
			return rfsys.Resolve(name)
		}

		// otherwise root files resolve to the same fs
		rfsys = fsys
		rname = name
		return
	}

	// TODO: make and use a ResolveFS interface, falling back to Sub if not supported
	dirfs, e := Sub(fsys, path.Dir(name))
	if e != nil {
		err = e
		return
	}

	if Equal(dirfs, fsys) {
		rfsys = fsys
		rname = name
		return
	}

	if subfs, ok := dirfs.(*SubdirFS); ok {
		rfsys = subfs.Fsys

		if Equal(subfs.Fsys, fsys) {
			rname = name
			return
		}

		rname, err = subfs.fullName("resolve", path.Base(name))
		return
	}

	rfsys = dirfs
	rname = path.Base(name)
	return
}
