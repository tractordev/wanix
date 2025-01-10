package fusekit

import (
	"errors"
	"io"
	iofs "io/fs"
	"os"
	"os/exec"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type fuseMount struct {
	path string
	*fuse.Server
}

func (m *fuseMount) Close() error {
	if m.Server == nil {
		exec.Command("umount", m.path).Run()
		return nil
	}
	return m.Server.Unmount()
}

func Mount(fsys iofs.FS, path string) (closer io.Closer, err error) {
	exec.Command("umount", path).Run()

	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, errors.New("unable to mkdir")
	}

	opts := &fs.Options{
		UID: uint32(os.Getuid()),
		GID: uint32(os.Getgid()),
	}
	opts.Debug = false

	server, err := fs.Mount(path, &Node{FS: fsys}, opts)
	if err != nil {
		return nil, err
	}

	return &fuseMount{Server: server, path: path}, nil
}
