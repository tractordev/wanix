package fs

import (
	"fmt"
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
		return 0, fmt.Errorf("%w: Seek", ErrNotSupported)
	}
	return s.Seek(offset, whence)
}

// ReadAt reads len(p) bytes into p starting at offset off in the file f.
// If f does not support ReadAt or Seek, it reads and discards bytes until the offset.
func ReadAt(f File, p []byte, off int64) (int, error) {
	if ra, ok := f.(io.ReaderAt); ok {
		return ra.ReadAt(p, off)
	}

	// Check if the file supports seeking
	if seeker, ok := f.(io.Seeker); ok {
		if _, err := seeker.Seek(off, io.SeekStart); err != nil {
			return 0, err
		}
		return f.Read(p)
	}

	// Emulate ReadAt by reading and discarding bytes up to off
	_, err := io.CopyN(io.Discard, f, off)
	if err != nil {
		return 0, err
	}

	return f.Read(p)
}

type SyncFile interface {
	File
	Sync() error
}

func Sync(f File) error {
	if sf, ok := f.(SyncFile); ok {
		return sf.Sync()
	}
	return fmt.Errorf("%w: Sync", ErrNotSupported)
}
