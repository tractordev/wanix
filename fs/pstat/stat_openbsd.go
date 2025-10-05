package pstat

import (
	"syscall"
)

func SysToStat(sys any) *Stat {
	if sys == nil {
		return &Stat{}
	}
	nativeStat := sys.(*syscall.Stat_t)
	return &Stat{
		Dev:     uint64(nativeStat.Dev),
		Ino:     nativeStat.Ino,
		Nlink:   uint64(nativeStat.Nlink),
		Mode:    uint32(nativeStat.Mode),
		Uid:     nativeStat.Uid,
		Gid:     nativeStat.Gid,
		Rdev:    uint64(nativeStat.Rdev),
		Size:    nativeStat.Size,
		Blksize: int64(nativeStat.Blksize),
		Blocks:  nativeStat.Blocks,
		Atim:    NsecToTimespec(syscall.TimespecToNsec(nativeStat.Atim)),
		Mtim:    NsecToTimespec(syscall.TimespecToNsec(nativeStat.Mtim)),
		Ctim:    NsecToTimespec(syscall.TimespecToNsec(nativeStat.Ctim)),
	}
}

func StatToSys(stat *Stat) any {
	return &syscall.Stat_t{
		Dev:     int32(stat.Dev),
		Ino:     stat.Ino,
		Nlink:   uint32(stat.Nlink),
		Mode:    uint32(stat.Mode),
		Uid:     stat.Uid,
		Gid:     stat.Gid,
		Rdev:    int32(stat.Rdev),
		Size:    stat.Size,
		Blksize: uint32(stat.Blksize),
		Blocks:  stat.Blocks,
		Atim:    syscall.NsecToTimespec(stat.Atim.Nsec),
		Mtim:    syscall.NsecToTimespec(stat.Mtim.Nsec),
		Ctim:    syscall.NsecToTimespec(stat.Ctim.Nsec),
	}
}
