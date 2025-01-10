package fs

import "io"

func Write(f File, data []byte) (int, error) {
	w, ok := f.(io.Writer)
	if !ok {
		return 0, ErrNotSupported
	}
	return w.Write(data)
}

func WriteAt(f File, data []byte, off int64) (int, error) {
	wa, ok := f.(io.WriterAt)
	if !ok {
		return 0, ErrNotSupported
	}
	return wa.WriteAt(data, off)
}

func Seek(f File, offset int64, whence int) (int64, error) {
	s, ok := f.(io.Seeker)
	if !ok {
		return 0, ErrNotSupported
	}
	return s.Seek(offset, whence)
}
