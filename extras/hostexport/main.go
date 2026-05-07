package main

import (
	"context"
	"log"
	"os"

	"github.com/hugelgupf/p9/p9"
	"go.bug.st/serial"
	"tractor.dev/wanix"
	"tractor.dev/wanix/fs/localfs"
	"tractor.dev/wanix/fs/p9kit"
	"tractor.dev/wanix/fs/vfs"
	"tractor.dev/wanix/native"
)

func main() {
	log.SetFlags(log.Lshortfile)
	fsys := vfs.New(context.Background())

	lfsys, err := localfs.New("/")
	if err != nil {
		log.Fatal(err)
	}
	if err := fsys.Bind(lfsys, ".", "."); err != nil {
		log.Fatal(err)
	}

	tfsys := wanix.NewTaskFS()
	tfsys.Register("native", &native.ExecDriver{})
	if err := fsys.Bind(tfsys, ".", "task"); err != nil {
		log.Fatal(err)
	}

	var opts []p9.ServerOpt
	// if os.Getenv("DEBUG") != "" {
	// opts = append(opts, p9.WithServerLogger(log.New(os.Stderr, "", log.LstdFlags)))
	// }
	s := p9.NewServer(p9kit.Attacher(fsys), opts...)

	exportdev := os.Getenv("EXPORTDEV")
	if exportdev == "" {
		if err := s.Handle(os.Stdin, os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}

	mode := &serial.Mode{
		BaudRate: 115200,
	}
	dev, err := serial.Open(exportdev, mode)
	if err != nil {
		log.Fatal(err)
	}
	defer dev.Close()

	if err := s.Handle(dev, dev); err != nil {
		log.Fatal(err)
	}
}
