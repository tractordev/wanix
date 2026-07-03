package fskit

import (
	"context"

	"tractor.dev/wanix/fs"
)

type OpenFunc func(ctx context.Context, name string) (fs.File, error)

var _ fs.OpenFileContextFS = OpenFunc(nil)

func (f OpenFunc) Open(name string) (fs.File, error) {
	return f(context.Background(), name)
}

func (f OpenFunc) OpenContext(ctx context.Context, name string) (fs.File, error) {
	return f(ctx, name)
}

func (f OpenFunc) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	return f(context.Background(), name)
}

func (f OpenFunc) OpenFileContext(ctx context.Context, name string, flag int, perm fs.FileMode) (fs.File, error) {
	return f(ctx, name)
}

// todo: this shouldn't be needed, but otherwise >> appends to files fail
func (f OpenFunc) Chmod(name string, mode fs.FileMode) (err error) {
	return nil
}
