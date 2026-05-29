package pstat

import "io/fs"

// UIDGIDProvider is implemented by file-info objects that carry explicit
// ownership information (e.g. in-memory nodes) independently of the
// underlying syscall.Stat_t.
type UIDGIDProvider interface {
	GetUID() int
	GetGID() int
}


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
	s.Nlink = 1
	if fi.IsDir() {
		s.Nlink = 2
	}
	s.Size = fi.Size()
	s.Mode = uint32(fi.Mode())
	s.Mtim = Timespec{
		Sec:  fi.ModTime().Unix(),
		Nsec: fi.ModTime().UnixNano(),
	}
	// Override uid/gid from the node if it carries explicit ownership.
	if p, ok := fi.(UIDGIDProvider); ok {
		s.Uid = uint32(p.GetUID())
		s.Gid = uint32(p.GetGID())
	}
	return s
}
