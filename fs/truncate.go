package fs

import (
	"errors"
	"fmt"
)

type TruncateFS interface {
	FS
	Truncate(name string, size int64) error
}

func Truncate(fsys FS, name string, size int64) error {
	if t, ok := fsys.(TruncateFS); ok {
		return t.Truncate(name, size)
	}

	rfsys, rname, err := ResolveTo[TruncateFS](fsys, ContextFor(fsys), name)
	if err == nil {
		return rfsys.Truncate(rname, size)
	}
	if !errors.Is(err, ErrNotSupported) {
		return opErr(fsys, name, "truncate", err)
	}

	b, err := ReadFile(fsys, name)
	if err != nil {
		fmt.Println("truncate:", name, size, "read error:", err)
		// If file doesn't exist (or anything else), create empty file of requested size
		return WriteFile(fsys, name, make([]byte, size), 0)
	}

	if size > int64(len(b)) {
		fullb := make([]byte, size)
		copy(fullb, b)
		return WriteFile(fsys, name, fullb, 0)
	}

	return WriteFile(fsys, name, b[:size], 0)
}
