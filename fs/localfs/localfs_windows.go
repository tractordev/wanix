package localfs

import (
	"context"

	"tractor.dev/wanix/fs"
)

func (fsys *FS) SetXattr(ctx context.Context, name string, attr string, data []byte, flags int) error {
	return fs.ErrNotSupported
}

func (fsys *FS) GetXattr(ctx context.Context, name string, attr string) ([]byte, error) {
	return nil, fs.ErrNotSupported
}

func (fsys *FS) ListXattrs(ctx context.Context, name string) ([]string, error) {
	return nil, fs.ErrNotSupported
}

func (fsys *FS) RemoveXattr(ctx context.Context, name string, attr string) error {
	return fs.ErrNotSupported
}
