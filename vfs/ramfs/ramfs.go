package ramfs

import (
	"context"
	"io/fs"

	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/memfs"
	"tractor.dev/wanix/vfs"
)

// Allocator allows binding a fresh MemFS per bind operation.
type Allocator struct{}

func (a *Allocator) Open(name string) (fs.File, error) {
	return a.OpenContext(context.Background(), name)
}

func (a *Allocator) OpenContext(ctx context.Context, name string) (fs.File, error) {
	return fskit.RawNode(name, 0644).OpenContext(ctx, name)
}

func (a *Allocator) BindAllocFS(name string) (fs.FS, error) {
	return memfs.New(), nil
}

var _ vfs.BindAllocator = (*Allocator)(nil)
