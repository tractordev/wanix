package fusekit

import (
	"context"
	"log"
	"path/filepath"
	"syscall"
	"time"

	iofs "tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/pstat"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type node struct {
	fs.Inode
	rootfs iofs.FS // The root filesystem for cross-directory operations
	fs     iofs.FS // Sub-filesystem for this node
	path   string  // Full path from root
	ctx    context.Context
}

var _ = (fs.NodeGetattrer)((*node)(nil))

func (n *node) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	// log.Println("getattr", n.path)

	fi, err := iofs.StatContext(n.ctx, n.fs, ".")
	if err != nil {
		return sysErrno(err)
	}
	applyStat(&out.Attr, fi)

	return 0
}

var _ = (fs.NodeSetattrer)((*node)(nil))

func (n *node) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	// log.Println("setattr", n.path, in.Valid)

	// Handle different attribute changes based on Valid bitmask
	if in.Valid&fuse.FATTR_MODE != 0 {
		if err := iofs.Chmod(n.fs, ".", iofs.FileMode(in.Mode)); err != nil {
			return sysErrno(err)
		}
	}

	if in.Valid&(fuse.FATTR_UID|fuse.FATTR_GID) != 0 {
		uid := int(-1)
		gid := int(-1)
		if in.Valid&fuse.FATTR_UID != 0 {
			uid = int(in.Uid)
		}
		if in.Valid&fuse.FATTR_GID != 0 {
			gid = int(in.Gid)
		}
		if err := iofs.Chown(n.fs, ".", uid, gid); err != nil {
			return sysErrno(err)
		}
	}

	if in.Valid&fuse.FATTR_SIZE != 0 {
		if err := iofs.Truncate(n.fs, ".", int64(in.Size)); err != nil {
			return sysErrno(err)
		}
	}

	if in.Valid&(fuse.FATTR_ATIME|fuse.FATTR_MTIME) != 0 {
		var atime, mtime time.Time
		now := time.Now()

		// Handle access time
		if in.Valid&fuse.FATTR_ATIME != 0 {
			if in.Valid&fuse.FATTR_ATIME_NOW != 0 {
				atime = now
			} else {
				atime = time.Unix(int64(in.Atime), int64(in.Atimensec))
			}
		} else {
			// Keep existing atime - get it from stat
			fi, err := iofs.StatContext(n.ctx, n.fs, ".")
			if err != nil {
				return sysErrno(err)
			}
			atime = fi.ModTime() // Note: may not have separate atime
		}

		// Handle modification time
		if in.Valid&fuse.FATTR_MTIME != 0 {
			if in.Valid&fuse.FATTR_MTIME_NOW != 0 {
				mtime = now
			} else {
				mtime = time.Unix(int64(in.Mtime), int64(in.Mtimensec))
			}
		} else {
			// Keep existing mtime
			fi, err := iofs.StatContext(n.ctx, n.fs, ".")
			if err != nil {
				return sysErrno(err)
			}
			mtime = fi.ModTime()
		}

		if err := iofs.Chtimes(n.fs, ".", atime, mtime); err != nil {
			return sysErrno(err)
		}
	}

	// Get updated attributes
	fi, err := iofs.StatContext(n.ctx, n.fs, ".")
	if err != nil {
		return sysErrno(err)
	}
	applyStat(&out.Attr, fi)

	return 0
}

var _ = (fs.NodeReaddirer)((*node)(nil))

func (n *node) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	// log.Println("readdir", n.path)

	entries, err := iofs.ReadDirContext(n.ctx, n.fs, ".")
	if err != nil {
		return nil, sysErrno(err)
	}

	var fentries []fuse.DirEntry
	for _, entry := range entries {
		entryPath := filepath.Join(n.path, entry.Name())

		// Try to get FileInfo for real inode numbers
		var fi iofs.FileInfo
		if infoEntry, err := entry.Info(); err == nil {
			fi = infoEntry
		}

		entryMode := pstat.FileModeToUnixMode(entry.Type())
		// log.Printf("readdir %s: entry=%s type=0%o unixMode=0%o isDir=%v",
		// 	n.path, entry.Name(), entry.Type(), entryMode, entry.IsDir())

		fentries = append(fentries, fuse.DirEntry{
			Name: entry.Name(),
			Mode: entryMode,
			Ino:  getIno(entryPath, fi),
		})
	}

	return fs.NewListDirStream(fentries), 0
}

var _ = (fs.NodeOpendirer)((*node)(nil))

func (r *node) Opendir(ctx context.Context) syscall.Errno {
	// log.Println("opendir", r.path)
	return 0
}

var _ = (fs.NodeLookuper)((*node)(nil))

func (n *node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	// Use lstat to not follow symlinks during lookup
	fi, err := iofs.LstatContext(n.ctx, n.fs, name)
	if err != nil {
		return nil, sysErrno(err)
	}

	applyStat(&out.Attr, fi)
	// log.Printf("lookup %s/%s: fi.Mode=0%o out.Attr.Mode=0%o isDir=%v isSymlink=%v",
	// 	n.path, name, fi.Mode(), out.Attr.Mode, fi.IsDir(), fi.Mode()&iofs.ModeSymlink != 0)

	subfs, err := iofs.Sub(n.fs, name)
	if err != nil {
		return nil, sysErrno(err)
	}

	// Determine file type from mode
	mode := fuse.S_IFREG
	if fi.IsDir() {
		mode = fuse.S_IFDIR
	} else if fi.Mode()&iofs.ModeSymlink != 0 {
		mode = fuse.S_IFLNK
	}

	return n.Inode.NewPersistentInode(ctx, &node{
		rootfs: n.rootfs,
		ctx:    n.ctx,
		fs:     subfs,
		path:   filepath.Join(n.path, name),
	}, fs.StableAttr{
		Mode: uint32(mode),
		Ino:  getIno(filepath.Join(n.path, name), fi),
	}), 0
}

var _ = (fs.NodeCreater)((*node)(nil))

func (n *node) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	// log.Println("create", n.path, name, flags, mode)
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
		rootfs: n.rootfs,
		ctx:    n.ctx,
		fs:     subfs,
		path:   filepath.Join(n.path, name),
	}, fs.StableAttr{
		Mode: uint32(outMode),
		Ino:  getIno(filepath.Join(n.path, name), fi),
	}), &handle{file: f, path: n.path}, fuse.FOPEN_DIRECT_IO, 0
}

var _ = (fs.NodeOpener)((*node)(nil))

func (n *node) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	// log.Println("open", n.path, strings.Join(openFlags(flags), "|"))

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

var _ = (fs.NodeMkdirer)((*node)(nil))

func (n *node) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	// log.Println("mkdir", n.path, name, mode)

	if err := iofs.Mkdir(n.fs, name, iofs.FileMode(mode)); err != nil {
		return nil, sysErrno(err)
	}

	fi, err := iofs.StatContext(n.ctx, n.fs, name)
	if err != nil {
		return nil, sysErrno(err)
	}

	applyStat(&out.Attr, fi)

	subfs, err := iofs.Sub(n.fs, name)
	if err != nil {
		return nil, sysErrno(err)
	}

	return n.Inode.NewPersistentInode(ctx, &node{
		rootfs: n.rootfs,
		ctx:    n.ctx,
		fs:     subfs,
		path:   filepath.Join(n.path, name),
	}, fs.StableAttr{
		Mode: uint32(fuse.S_IFDIR),
		Ino:  getIno(filepath.Join(n.path, name), fi),
	}), 0
}

var _ = (fs.NodeUnlinker)((*node)(nil))

func (n *node) Unlink(ctx context.Context, name string) syscall.Errno {
	// log.Println("unlink", n.path, name)

	if err := iofs.Remove(n.fs, name); err != nil {
		return sysErrno(err)
	}

	return 0
}

var _ = (fs.NodeRmdirer)((*node)(nil))

func (n *node) Rmdir(ctx context.Context, name string) syscall.Errno {
	// log.Println("rmdir", n.path, name)

	if err := iofs.Remove(n.fs, name); err != nil {
		return sysErrno(err)
	}

	return 0
}

var _ = (fs.NodeRenamer)((*node)(nil))

func (n *node) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	// Get the target parent node
	newParentNode, ok := newParent.(*node)
	if !ok {
		return syscall.EINVAL
	}

	// Construct full paths from root for both source and destination
	oldPath := filepath.Join(n.path, name)
	newPath := filepath.Join(newParentNode.path, newName)

	// log.Println("rename", oldPath, "->", newPath, "flags", flags)

	// Use root filesystem to perform rename across any directory structure
	if err := iofs.Rename(n.rootfs, oldPath, newPath); err != nil {
		return sysErrno(err)
	}

	return 0
}

var _ = (fs.NodeSymlinker)((*node)(nil))

func (n *node) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	// log.Println("symlink", n.path, target, name)

	if err := iofs.Symlink(n.fs, target, name); err != nil {
		return nil, sysErrno(err)
	}

	// Use lstat to get symlink attributes without following it
	fi, err := iofs.LstatContext(n.ctx, n.fs, name)
	if err != nil {
		return nil, sysErrno(err)
	}

	applyStat(&out.Attr, fi)

	subfs, err := iofs.Sub(n.fs, name)
	if err != nil {
		return nil, sysErrno(err)
	}

	return n.Inode.NewPersistentInode(ctx, &node{
		rootfs: n.rootfs,
		ctx:    n.ctx,
		fs:     subfs,
		path:   filepath.Join(n.path, name),
	}, fs.StableAttr{
		Mode: uint32(fuse.S_IFLNK),
		Ino:  getIno(filepath.Join(n.path, name), fi),
	}), 0
}

var _ = (fs.NodeReadlinker)((*node)(nil))

func (n *node) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	// log.Println("readlink", n.path)

	target, err := iofs.Readlink(n.fs, ".")
	if err != nil {
		return nil, sysErrno(err)
	}

	return []byte(target), 0
}

var _ = (fs.NodeGetxattrer)((*node)(nil))

func (n *node) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	// log.Println("getxattr", n.path, attr)

	data, err := iofs.GetXattr(n.ctx, n.fs, ".", attr)
	if err != nil {
		return 0, sysErrno(err)
	}

	// If dest is nil, just return the size
	if dest == nil {
		return uint32(len(data)), 0
	}

	// If dest is too small, return ERANGE
	if len(dest) < len(data) {
		return uint32(len(data)), syscall.ERANGE
	}

	// Copy data to dest
	copy(dest, data)
	return uint32(len(data)), 0
}

var _ = (fs.NodeSetxattrer)((*node)(nil))

func (n *node) Setxattr(ctx context.Context, attr string, data []byte, flags uint32) syscall.Errno {
	// log.Println("setxattr", n.path, attr, len(data), flags)

	if err := iofs.SetXattr(n.ctx, n.fs, ".", attr, data, int(flags)); err != nil {
		return sysErrno(err)
	}

	return 0
}

var _ = (fs.NodeListxattrer)((*node)(nil))

func (n *node) Listxattr(ctx context.Context, dest []byte) (uint32, syscall.Errno) {
	// log.Println("listxattr", n.path)

	attrs, err := iofs.ListXattrs(n.ctx, n.fs, ".")
	if err != nil {
		return 0, sysErrno(err)
	}

	// Format as null-terminated strings
	var totalSize uint32
	for _, attr := range attrs {
		totalSize += uint32(len(attr) + 1) // +1 for null terminator
	}

	// If dest is nil, just return the size
	if dest == nil {
		return totalSize, 0
	}

	// If dest is too small, return ERANGE
	if uint32(len(dest)) < totalSize {
		return totalSize, syscall.ERANGE
	}

	// Copy attrs to dest as null-terminated strings
	offset := 0
	for _, attr := range attrs {
		copy(dest[offset:], attr)
		offset += len(attr)
		dest[offset] = 0 // null terminator
		offset++
	}

	return totalSize, 0
}

var _ = (fs.NodeRemovexattrer)((*node)(nil))

func (n *node) Removexattr(ctx context.Context, attr string) syscall.Errno {
	// log.Println("removexattr", n.path, attr)

	if err := iofs.RemoveXattr(n.ctx, n.fs, ".", attr); err != nil {
		return sysErrno(err)
	}

	return 0
}

var _ = (fs.NodeCopyFileRanger)((*node)(nil))

func (n *node) CopyFileRange(ctx context.Context, fhIn fs.FileHandle, offIn uint64, dest *fs.Inode, fhOut fs.FileHandle, offOut uint64, len uint64, flags uint64) (uint32, syscall.Errno) {
	log.Println("copyfilerange", n.path, offIn, offOut, len)

	// Get the input and output file handles
	hIn, ok := fhIn.(*handle)
	if !ok {
		return 0, syscall.EINVAL
	}
	hOut, ok := fhOut.(*handle)
	if !ok {
		return 0, syscall.EINVAL
	}

	// Read from input file at offIn
	buf := make([]byte, len)
	nRead, err := iofs.ReadAt(hIn.file, buf, int64(offIn))
	if err != nil && nRead == 0 {
		return 0, sysErrno(err)
	}

	// Write to output file at offOut
	nWritten, err := iofs.WriteAt(hOut.file, buf[:nRead], int64(offOut))
	if err != nil {
		return uint32(nWritten), sysErrno(err)
	}

	return uint32(nWritten), 0
}

var _ = (fs.NodeLinker)((*node)(nil))

func (n *node) Link(ctx context.Context, target fs.InodeEmbedder, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	// log.Println("link", n.path, name)

	// Hard links are not supported by most custom filesystems
	// Return EOPNOTSUPP to indicate this feature is not available
	return nil, syscall.EOPNOTSUPP
}
