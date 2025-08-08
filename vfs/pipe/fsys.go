package pipe

import (
	"context"
	iofs "io/fs"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/vfs"
)

const (
	// DataFile is the name of the first pipe end file
	DataFile = "data"
	// Data1File is the name of the second pipe end file
	Data1File = "data1"
)

// PipeFS presents two files at its root: "data" and "data1",
// each representing one end of a full-duplex in-memory pipe.
//
// Opening "data" returns a stream file backed by Port 1.
// Opening "data1" returns a stream file backed by Port 2.
//
// Closing either file via PortFile.Close is a no-op; callers may close the
// underlying Port through the PortFile if they need to tear down the pipe.
type PipeFS struct {
	pf1 *PortFile
	pf2 *PortFile
}

// NewFS constructs a PipeFS and returns it along with both PortFiles for
// direct access to each end of the pipe.
func NewFS(block bool) (iofs.FS, *PortFile, *PortFile) {
	p1, p2 := New(block)
	pf1 := &PortFile{Port: p1, Name: DataFile}
	pf2 := &PortFile{Port: p2, Name: Data1File}
	return &PipeFS{pf1: pf1, pf2: pf2}, pf1, pf2
}

var _ fs.FS = (*PipeFS)(nil)
var _ fs.OpenContextFS = (*PipeFS)(nil)

func (p *PipeFS) Open(name string) (iofs.File, error) {
	return p.OpenContext(context.Background(), name)
}

func (p *PipeFS) OpenContext(ctx context.Context, name string) (iofs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	switch name {
	case ".":
		// directory listing with two entries
		dir := fskit.Entry(".", fs.ModeDir|0555)
		return fskit.DirFile(dir,
			fskit.Entry(DataFile, 0644),
			fskit.Entry(Data1File, 0644),
		), nil
	case DataFile:
		return p.pf1, nil
	case Data1File:
		return p.pf2, nil
	default:
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
}

// Allocator allows binding a fresh PipeFS per bind operation.
type Allocator struct{}

func (a *Allocator) Open(name string) (iofs.File, error) {
	return a.OpenContext(context.Background(), name)
}

func (a *Allocator) OpenContext(ctx context.Context, name string) (iofs.File, error) {
	return fskit.RawNode(name, 0644).OpenContext(ctx, name)
}

func (a *Allocator) BindAllocFS(name string) (iofs.FS, error) {
	fsys, _, _ := NewFS(false)
	return fsys, nil
}

var _ vfs.BindAllocator = (*Allocator)(nil)
