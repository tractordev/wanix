package fs

import (
	"fmt"
	"hash/fnv"
	"io"
	"reflect"
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
func WriteAt(f File, data []byte, off int64) (n int, err error) {
	// defer func() {
	// 	log.Printf("writeat %d %d: %v", off, len(data), err)
	// }()
	// Validate offset
	if off < 0 {
		return 0, fmt.Errorf("negative offset: %d", off)
	}

	// Try WriterAt interface first (most efficient)
	if wa, ok := f.(io.WriterAt); ok {
		return wa.WriteAt(data, off)
	}

	// Check if file supports writing at all
	_, ok := f.(io.Writer)
	if !ok {
		return 0, ErrPermission
	}

	// For offset 0, we can write directly
	if off == 0 {
		return Write(f, data)
	}

	// Seek to the desired position
	_, err = Seek(f, off, 0) // SEEK_SET = 0
	if err != nil {
		return 0, fmt.Errorf("seek to offset %d: %w", off, err)
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

type IdentityFile interface {
	File
	Identity() ID
}

type ID struct {
	Dev uint64
	Ino uint64
	Ptr uint64
	Sum uint64
}

type Namer interface {
	Name() string
}

func Identity(f any) ID {
	if ifi, ok := f.(IdentityFile); ok {
		return ifi.Identity()
	}

	// todo: get from stat

	if nf, ok := f.(Namer); ok {
		h := fnv.New64a()
		h.Write([]byte(nf.Name()))
		return ID{Sum: h.Sum64()}
	}

	rv := reflect.ValueOf(f)
	if rv.Kind() == reflect.Ptr {
		return ID{Ptr: uint64(rv.Pointer())}
	}

	if sf, ok := f.(fmt.Stringer); ok {
		h := fnv.New64a()
		h.Write([]byte(sf.String()))
		return ID{Sum: h.Sum64()}
	}

	panic("unable to get identity")
}

func SameFile(a, b File) bool {
	// log.Println("SameFile", a, b, Identity(a), Identity(b))
	return Identity(a) == Identity(b)
}
