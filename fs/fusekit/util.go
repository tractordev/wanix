package fusekit

import (
	"hash/fnv"
	iofs "io/fs"
	"os"
	"syscall"

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

func openFlags(flags uint32) []string {
	var flagStrs []string
	if flags&os.O_RDONLY != 0 {
		flagStrs = append(flagStrs, "O_RDONLY")
	}
	if flags&os.O_WRONLY != 0 {
		flagStrs = append(flagStrs, "O_WRONLY")
	}
	if flags&os.O_RDWR != 0 {
		flagStrs = append(flagStrs, "O_RDWR")
	}
	if flags&os.O_APPEND != 0 {
		flagStrs = append(flagStrs, "O_APPEND")
	}
	if flags&os.O_CREATE != 0 {
		flagStrs = append(flagStrs, "O_CREAT")
	}
	if flags&os.O_EXCL != 0 {
		flagStrs = append(flagStrs, "O_EXCL")
	}
	if flags&os.O_SYNC != 0 {
		flagStrs = append(flagStrs, "O_SYNC")
	}
	if flags&os.O_TRUNC != 0 {
		flagStrs = append(flagStrs, "O_TRUNC")
	}
	return flagStrs
}
