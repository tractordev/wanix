package main

import (
	"log"
	"os"

	"github.com/hugelgupf/p9/p9"
	"go.bug.st/serial"
	"tractor.dev/wanix"
	"tractor.dev/wanix/fs/localfs"
	"tractor.dev/wanix/fs/p9kit"
	"tractor.dev/wanix/native"
	"tractor.dev/wanix/term"
)

func main() {
	log.SetFlags(log.Lshortfile)

	taskfs := wanix.NewTaskFS()
	taskfs.Register("native", &native.ExecDriver{})
	root, err := wanix.NewRootWithTasks(taskfs)
	if err != nil {
		log.Fatal(err)
	}

	lfsys, err := localfs.New("/")
	if err != nil {
		log.Fatal(err)
	}
	if err := root.NS().Bind(lfsys, ".", "."); err != nil {
		log.Fatal(err)
	}

	termfs := term.New(nil)
	if err := root.NS().Bind(termfs, ".", "#term"); err != nil {
		log.Fatal(err)
	}

	if err := root.Bind("#task", "task"); err != nil {
		log.Fatal(err)
	}
	if err := root.Bind("#term", "term"); err != nil {
		log.Fatal(err)
	}

	var opts []p9.ServerOpt
	// if os.Getenv("DEBUG") != "" {
	// opts = append(opts, p9.WithServerLogger(log.New(os.Stderr, "", log.LstdFlags)))
	// }
	s := p9.NewServer(p9kit.Attacher(root.NS()), opts...)

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

	// first packet signals to start client export
	if _, err := dev.Write([]byte("!")); err != nil {
		log.Fatal(err)
	}

	if err := s.Handle(dev, dev); err != nil {
		log.Fatal(err)
	}
}
