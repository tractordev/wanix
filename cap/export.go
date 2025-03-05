package cap

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hugelgupf/p9/p9"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/p9kit"
)

func Export(fsys fs.FS) error {
	srv := p9.NewServer(p9kit.Attacher(fsys)) //, p9.WithServerLogger(ulog.Log)
	b, err := os.ReadFile("/cap/new/loopback")
	if err != nil {
		return err
	}
	id := strings.TrimSpace(string(b))
	time.Sleep(1 * time.Second) // kludge: wait for fsa readdir cache to expire
	f, err := os.Open(fmt.Sprintf("/cap/%s/loopback", id))
	if err != nil {
		return err
	}
	return srv.Handle(f, f)
}
