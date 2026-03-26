//go:build js && wasm

package tcp

import (
	"context"

	"tractor.dev/wanix/fs"
)

// Conn is not supported in js/wasm builds (no Go net stack available).
// The Service can still allocate ids, but operations will fail with ErrNotSupported.
type Conn struct{}

func newConn(_ string, _ *Service) *Conn { return &Conn{} }
func (c *Conn) shutdown()               {}

func (c *Conn) Open(name string) (fs.File, error) {
	return c.OpenContext(context.Background(), name)
}

func (c *Conn) OpenContext(ctx context.Context, name string) (fs.File, error) {
	return nil, fs.ErrNotSupported
}

func (c *Conn) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	return nil, name, fs.ErrNotSupported
}

