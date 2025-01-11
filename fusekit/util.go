package fusekit

import (
	"hash/fnv"
	iofs "io/fs"
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
	if flags&syscall.O_RDONLY != 0 {
		flagStrs = append(flagStrs, "O_RDONLY")
	}
	if flags&syscall.O_WRONLY != 0 {
		flagStrs = append(flagStrs, "O_WRONLY")
	}
	if flags&syscall.O_RDWR != 0 {
		flagStrs = append(flagStrs, "O_RDWR")
	}
	if flags&syscall.O_APPEND != 0 {
		flagStrs = append(flagStrs, "O_APPEND")
	}
	if flags&syscall.O_CREAT != 0 {
		flagStrs = append(flagStrs, "O_CREAT")
	}
	if flags&syscall.O_EXCL != 0 {
		flagStrs = append(flagStrs, "O_EXCL")
	}
	if flags&syscall.O_SYNC != 0 {
		flagStrs = append(flagStrs, "O_SYNC")
	}
	if flags&syscall.O_TRUNC != 0 {
		flagStrs = append(flagStrs, "O_TRUNC")
	}
	return flagStrs
}
