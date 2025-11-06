package fskit

import (
	"context"
	"io/fs"
)

type OpenFunc func(ctx context.Context, name string) (fs.File, error)

func (f OpenFunc) Open(name string) (fs.File, error) {
	return f(context.Background(), name)
}

func (f OpenFunc) OpenContext(ctx context.Context, name string) (fs.File, error) {
	return f(ctx, name)
}

func (f OpenFunc) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	return f(context.Background(), name)
}
