package pipe

import (
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

// PortFile wraps a Port and exposes it as an fs.File.
// It models a stream-like, non-seekable special file (named pipe).
//
// Semantics:
// - Read/Write delegate to the underlying Port
// - ReadAt returns EOF for any non-zero offset, otherwise defers to Read
// - WriteAt ignores offset and defers to Write
// - Close is a no-op; callers must close the underlying Port explicitly
// - Stat returns a FileInfo identifying this as a named pipe
//
// Note: fs.File does not require Write, ReadAt, or WriteAt, but providing them
// keeps this consistent with other stream wrappers in this repo.
type PortFile struct {
	Port *Port
	Name string
}

var _ fs.File = (*PortFile)(nil)

func (pf *PortFile) Close() error { return nil }

func (pf *PortFile) Read(b []byte) (int, error) { return pf.Port.Read(b) }

func (pf *PortFile) Stat() (fs.FileInfo, error) {
	return fskit.Entry(pf.Name, fs.FileMode(0644), int64(pf.Port.Size())), nil
}

// Optional stream-friendly methods
func (pf *PortFile) ReadAt(b []byte, off int64) (int, error) {
	// if off > 0 {
	// 	return 0, io.EOF
	// }
	return pf.Read(b)
}

func (pf *PortFile) Write(b []byte) (int, error) { return pf.Port.Write(b) }

func (pf *PortFile) WriteAt(b []byte, off int64) (int, error) { return pf.Write(b) }
