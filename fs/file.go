package fs

import "io"

func Write(f File, data []byte) (int, error) {
	w, ok := f.(io.Writer)
	if !ok {
		return 0, ErrPermission
	}
	return w.Write(data)
}

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

func Seek(f File, offset int64, whence int) (int64, error) {
	s, ok := f.(io.Seeker)
	if !ok {
		return 0, ErrNotSupported
	}
	return s.Seek(offset, whence)
}
