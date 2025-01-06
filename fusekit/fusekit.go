package fusekit

import (
	"context"
	"hash/fnv"
	"io"
	iofs "io/fs"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

func fakeIno(s string) uint64 {
	h := fnv.New64a() // FNV-1a 64-bit hash
	h.Write([]byte(s))
	return h.Sum64()
}

func applyStat(out *fuse.Attr, fi iofs.FileInfo) {
	stat := fi.Sys()
	if s, ok := stat.(*syscall.Stat_t); ok {
		out.FromStat(s)
		return
	}
	out.Mtime = uint64(fi.ModTime().Unix())
	out.Mtimensec = uint32(fi.ModTime().UnixNano())
	out.Mode = uint32(fi.Mode())
	out.Size = uint64(fi.Size())
}

type Node struct {
	fs.Inode
	FS   iofs.FS
	path string
}

var _ = (fs.NodeGetattrer)((*Node)(nil))

func (n *Node) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	log.Println("getattr", n.path)

	fi, err := iofs.Stat(n.FS, ".")
	if err != nil {
		return sysErrno(err)
	}
	applyStat(&out.Attr, fi)

	return 0
}

var _ = (fs.NodeReaddirer)((*Node)(nil))

func (n *Node) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	log.Println("readdir", n.path)

	entries, err := iofs.ReadDir(n.FS, ".")
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

var _ = (fs.NodeOpendirer)((*Node)(nil))

func (r *Node) Opendir(ctx context.Context) syscall.Errno {
	log.Println("opendir", r.path)
	return 0
}

var _ = (fs.NodeLookuper)((*Node)(nil))

func (n *Node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Println("lookup", n.path, name)

	fi, err := iofs.Stat(n.FS, name)
	if err != nil {
		return nil, sysErrno(err)
	}

	applyStat(&out.Attr, fi)

	subfs, err := iofs.Sub(n.FS, name)
	if err != nil {
		return nil, sysErrno(err)
	}

	mode := fuse.S_IFREG
	if fi.IsDir() {
		mode = fuse.S_IFDIR
	}

	return n.Inode.NewPersistentInode(ctx, &Node{
		FS:   subfs,
		path: filepath.Join(n.path, name),
	}, fs.StableAttr{
		Mode: uint32(mode),
		Ino:  fakeIno(filepath.Join(n.path, name)),
	}), 0
}

var _ = (fs.NodeOpener)((*Node)(nil))

func (n *Node) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	log.Println("open", n.path)

	f, err := n.FS.Open(".") // should be OpenFile
	if err != nil {
		return nil, 0, sysErrno(err)
	}

	return &handle{file: f, path: n.path}, 0, 0
}

type handle struct {
	file iofs.File
	path string
}

var _ = (fs.FileReader)((*handle)(nil))

func (h *handle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	log.Println("read", h.path)

	if ra, ok := h.file.(io.ReaderAt); ok {
		n, err := ra.ReadAt(dest, off)
		if err != nil {
			return nil, sysErrno(err)
		}
		return fuse.ReadResultData(dest[:n]), 0
	}

	if off > 0 {
		return nil, sysErrno(iofs.ErrPermission)
	}

	n, err := h.file.Read(dest)
	if err != nil {
		return nil, sysErrno(err)
	}

	return fuse.ReadResultData(dest[:n]), 0
}

var _ = (fs.FileFlusher)((*handle)(nil))

func (h *handle) Flush(ctx context.Context) syscall.Errno {
	log.Println("flush", h.path)

	if err := h.file.Close(); err != nil {
		return sysErrno(err)
	}

	return 0
}

func sysErrno(err error) syscall.Errno {
	log.Println("ERR:", err)
	switch err {
	case nil:
		return syscall.Errno(0)
	case os.ErrPermission:
		return syscall.EPERM
	case os.ErrExist:
		return syscall.EEXIST
	case os.ErrNotExist:
		return syscall.ENOENT
	case os.ErrInvalid:
		return syscall.EINVAL
	}

	switch t := err.(type) {
	case syscall.Errno:
		return t
	case *os.SyscallError:
		return t.Err.(syscall.Errno)
	case *os.PathError:
		return sysErrno(t.Err)
	case *os.LinkError:
		return sysErrno(t.Err)
	}
	log.Println("!! unsupported error type:", err)
	return syscall.EINVAL
}
