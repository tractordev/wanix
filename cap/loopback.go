package cap

import (
	"context"
	"net"

	"github.com/hugelgupf/p9/p9"
	"github.com/u-root/uio/ulog"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/p9kit"
)

func loopbackAllocator() Allocator {
	return func(r *Resource) (Mounter, error) {
		loopbackA, loopbackB := net.Pipe()
		r.Extra["loopback"] = fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			return &fskit.FuncFile{
				Node: fskit.Entry(name, 0644, loopbackA),
				ReadFunc: func(n *fskit.Node) (err error) {
					delete(r.Extra, "loopback")
					r.fs, err = r.mounter(nil)
					return
				},
			}, nil
		})
		return func(_ []string) (fs.FS, error) {
			return p9kit.ClientFS(loopbackB, "", p9.WithClientLogger(ulog.Log))
		}, nil
	}
}
