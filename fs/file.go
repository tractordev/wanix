package fs

import (
	"io"
)

// Write writes data to the file.
func Write(f File, data []byte) (int, error) {
	w, ok := f.(io.Writer)
	if !ok {
		return 0, ErrPermission
	}
	return w.Write(data)
}

// WriteAt writes data to the file at the given offset.
func WriteAt(f File, data []byte, off int64) (int, error) {
	_, ok := f.(io.Writer)
	if !ok {
		return 0, ErrPermission
	}
	wa, ok := f.(io.WriterAt)
	if ok {
		return wa.WriteAt(data, off)
	}
	if off > 0 {
		_, err := Seek(f, off, 0)
		if err != nil {
			return 0, err
		}
	}
	return Write(f, data)
}

// Seek seeks to the given offset and whence.
func Seek(f File, offset int64, whence int) (int64, error) {
	s, ok := f.(io.Seeker)
	if !ok {
		return 0, ErrNotSupported
	}
	return s.Seek(offset, whence)
}
