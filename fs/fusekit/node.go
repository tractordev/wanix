package fusekit

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"syscall"

	iofs "tractor.dev/wanix/fs"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// TODO: mkdir

type node struct {
	fs.Inode
	fs   iofs.FS
	path string
	ctx  context.Context
}

var _ = (fs.NodeGetattrer)((*node)(nil))

func (n *node) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	log.Println("getattr", n.path)

	fi, err := iofs.StatContext(n.ctx, n.fs, ".")
	if err != nil {
		return sysErrno(err)
	}
	applyStat(&out.Attr, fi)

	return 0
}

var _ = (fs.NodeSetattrer)((*node)(nil))

func (n *node) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	log.Println("setattr", n.path)
	out.Size = in.Size
	return 0
}

var _ = (fs.NodeReaddirer)((*node)(nil))

func (n *node) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	log.Println("readdir", n.path)

	entries, err := iofs.ReadDirContext(n.ctx, n.fs, ".")
	if err != nil {
		return nil, sysErrno(err)
	}

	var fentries []fuse.DirEntry
	for _, entry := range entries {
		fentries = append(fentries, fuse.DirEntry{
			Name: entry.Name(),
			Mode: uint32(entry.Type()),
			Ino:  fakeIno(filepath.Join(n.path, entry.Name())),
		})
	}

	return fs.NewListDirStream(fentries), 0
}

var _ = (fs.NodeOpendirer)((*node)(nil))

func (r *node) Opendir(ctx context.Context) syscall.Errno {
	log.Println("opendir", r.path)
	return 0
}

var _ = (fs.NodeLookuper)((*node)(nil))

func (n *node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Println("lookup", n.path, name)

	fi, err := iofs.StatContext(n.ctx, n.fs, name)
	if err != nil {
		return nil, sysErrno(err)
	}

	applyStat(&out.Attr, fi)

	subfs, err := iofs.Sub(n.fs, name)
	if err != nil {
		return nil, sysErrno(err)
	}

	mode := fuse.S_IFREG
	if fi.IsDir() {
		mode = fuse.S_IFDIR
	}

	return n.Inode.NewPersistentInode(ctx, &node{
		ctx:  n.ctx,
		fs:   subfs,
		path: filepath.Join(n.path, name),
	}, fs.StableAttr{
		Mode: uint32(mode),
		Ino:  fakeIno(filepath.Join(n.path, name)),
	}), 0
}

var _ = (fs.NodeCreater)((*node)(nil))

func (n *node) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	log.Println("create", n.path, name, flags, mode)
	// TODO: check if we can create a file on our n.fs
	// TODO: do we need to check if creating a dir?

	f, err := iofs.Create(n.fs, name)
	if err != nil {
		return nil, 0, 0, sysErrno(err)
	}
	fi, err := f.Stat()
	if err != nil {
		return nil, 0, 0, sysErrno(err)
	}
	// if err := f.Close(); err != nil {
	// 	return nil, 0, 0, sysErrno(err)
	// }

	applyStat(&out.Attr, fi)

	subfs, err := iofs.Sub(n.fs, name)
	if err != nil {
		return nil, 0, 0, sysErrno(err)
	}

	outMode := fuse.S_IFREG
	if fi.IsDir() {
		outMode = fuse.S_IFDIR
	}

	return n.Inode.NewPersistentInode(ctx, &node{
		ctx:  n.ctx,
		fs:   subfs,
		path: filepath.Join(n.path, name),
	}, fs.StableAttr{
		Mode: uint32(outMode),
		Ino:  fakeIno(filepath.Join(n.path, name)),
	}), &handle{file: f, path: n.path}, fuse.FOPEN_DIRECT_IO, 0
}

var _ = (fs.NodeOpener)((*node)(nil))

func (n *node) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	log.Println("open", n.path, strings.Join(openFlags(flags), "|"))

	var f iofs.File
	var err error
	if flags&syscall.O_CREAT != 0 {
		f, err = iofs.Create(n.fs, ".")
		if err != nil {
			return nil, 0, sysErrno(err)
		}
	} else {
		f, err = iofs.OpenContext(n.ctx, n.fs, ".")
		if err != nil {
			return nil, 0, sysErrno(err)
		}
	}

	return &handle{file: f, path: n.path}, fuse.FOPEN_DIRECT_IO, 0
}
