package fusekit

import (
	"hash/fnv"
	iofs "io/fs"
	"os"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fuse"
	"tractor.dev/wanix/fs/pstat"
)

func fakeIno(s string) uint64 {
	h := fnv.New64a() // FNV-1a 64-bit hash
	h.Write([]byte(s))
	return h.Sum64()
}

// getIno tries to extract a real inode number from the FileInfo,
// falling back to a hash-based inode for virtual filesystems.
func getIno(path string, fi iofs.FileInfo) uint64 {
	if fi == nil {
		return fakeIno(path)
	}

	// Try to extract real inode from underlying filesystem
	if sys := fi.Sys(); sys != nil {
		// Try Unix syscall.Stat_t (works on Unix systems with real filesystems)
		if s, ok := sys.(*syscall.Stat_t); ok && s.Ino != 0 {
			return s.Ino
		}
		// Try wanix portable stat (used by virtualizing filesystems)
		if s, ok := sys.(*pstat.Stat); ok && s.Ino != 0 {
			return s.Ino
		}
	}

	// Fallback to hash-based inode for virtual/remote filesystems
	return fakeIno(path)
}

func applyStat(out *fuse.Attr, fi iofs.FileInfo) {
	stat := fi.Sys()
	if s, ok := stat.(*syscall.Stat_t); ok {
		out.FromStat(s)
		return
	}
	out.Mtime = uint64(fi.ModTime().Unix())
	out.Mtimensec = uint32(fi.ModTime().UnixNano())
	out.Mode = pstat.FileModeToUnixMode(fi.Mode())
	out.Size = uint64(fi.Size())
}

func openFlags(flags uint32) []string {
	var flagStrs []string
	if flags&uint32(os.O_RDONLY) != 0 {
		flagStrs = append(flagStrs, "O_RDONLY")
	}
	if flags&uint32(os.O_WRONLY) != 0 {
		flagStrs = append(flagStrs, "O_WRONLY")
	}
	if flags&uint32(os.O_RDWR) != 0 {
		flagStrs = append(flagStrs, "O_RDWR")
	}
	if flags&uint32(os.O_APPEND) != 0 {
		flagStrs = append(flagStrs, "O_APPEND")
	}
	if flags&uint32(os.O_CREATE) != 0 {
		flagStrs = append(flagStrs, "O_CREAT")
	}
	if flags&uint32(os.O_EXCL) != 0 {
		flagStrs = append(flagStrs, "O_EXCL")
	}
	if flags&uint32(os.O_SYNC) != 0 {
		flagStrs = append(flagStrs, "O_SYNC")
	}
	if flags&uint32(os.O_TRUNC) != 0 {
		flagStrs = append(flagStrs, "O_TRUNC")
	}
	return flagStrs
}
