package fs

import (
	"errors"
	"io"
	"os"
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

	// Fallback: Try to open and truncate via file operations
	// This is better than Create() because it preserves the file identity
	f, err := OpenFile(fsys, name, os.O_RDWR, 0)
	if err != nil {
		// If file doesn't exist, create it with the requested size
		if os.IsNotExist(err) {
			return WriteFile(fsys, name, make([]byte, size), 0)
		}
		return err
	}
	defer f.Close()

	// Check if file implements Truncate directly
	if tf, ok := f.(interface{ Truncate(int64) error }); ok {
		return tf.Truncate(size)
	}

	// Fallback: Read, resize, seek, write
	// This is inefficient but preserves file handle identity
	b, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	var newData []byte
	if size > int64(len(b)) {
		// Extend with null bytes
		newData = make([]byte, size)
		copy(newData, b)
	} else {
		// Truncate
		newData = b[:size]
	}

	// Seek to beginning and write
	if seeker, ok := f.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return err
		}
	}

	if writer, ok := f.(io.Writer); ok {
		n, err := writer.Write(newData)
		if err != nil {
			return err
		}
		if n != len(newData) {
			return io.ErrShortWrite
		}
		return nil
	}

	return ErrNotSupported
}
