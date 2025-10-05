package pstat

import "io/fs"

// NOTE: taken from amd64 Linux
type Timespec struct {
	Sec  int64
	Nsec int64
}

type Stat struct {
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

// NsecToTimespec converts a number of nanoseconds into a Timespec.
func NsecToTimespec(nsec int64) Timespec {
	sec := nsec / 1e9
	nsec = nsec % 1e9
	if nsec < 0 {
		nsec += 1e9
		sec--
	}
	return Timespec{Sec: sec, Nsec: nsec}
}

func FileInfoToStat(fi fs.FileInfo) *Stat {
	s := SysToStat(fi.Sys())
	s.Size = fi.Size()
	s.Mode = uint32(fi.Mode())
	s.Mtim = Timespec{
		Sec:  fi.ModTime().Unix(),
		Nsec: fi.ModTime().UnixNano(),
	}
	return s
}
