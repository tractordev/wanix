package fs

import (
	"errors"
	"io"
)

type WriteFileFS interface {
	FS
	WriteFile(filename string, data []byte, perm FileMode) error
}

func WriteFile(fsys FS, filename string, data []byte, perm FileMode) error {
	if w, ok := fsys.(WriteFileFS); ok {
		return w.WriteFile(filename, data, perm)
	}

	f, err := Create(fsys, filename)
	if errors.Is(err, ErrNotSupported) {
		var e error
		f, e = fsys.Open(filename)
		if errors.Is(e, ErrNotExist) {
			// ok go back to unsupported error
			return err //fmt.Errorf("create: %w on %s", ErrNotSupported, reflect.TypeOf(fsys))
		}
		if e != nil {
			return e
		}
	} else if err != nil {
		return err
	}
	n, err := Write(f, data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	if err1 := f.Close(); err == nil {
		err = err1
	}
	// TODO: use perm?
	return err
}
