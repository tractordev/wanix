//go:build js || wasip1 || wasm

package stat

// NOTE: taken from amd64 Linux
type Timespec struct {
	Sec  int64
	Nsec int64
}

type Stat_t struct {
	Dev     uint64
	Ino     uint64
	Nlink   uint64
	Mode    uint32
	Uid     uint32
	Gid     uint32
	Rdev    uint64
	Size    int64
	Blksize int64
	Blocks  int64
	Atim    Timespec
	Mtim    Timespec
	Ctim    Timespec
}
