package fsys

import (
	"os"

	"github.com/hugelgupf/p9/p9"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/p9kit"
)

func Export(fsys fs.FS) error {
	srv := p9.NewServer(p9kit.Attacher(fsys)) //, p9.WithServerLogger(ulog.Log)
	f, err := os.Open("/fsys/export")
	if err != nil {
		return err
	}
	return srv.Handle(f, f)
}
