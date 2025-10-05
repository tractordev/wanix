//go:build freebsd || darwin || netbsd
// +build freebsd darwin netbsd

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
		Atim:    NsecToTimespec(syscall.TimespecToNsec(nativeStat.Atimespec)),
		Mtim:    NsecToTimespec(syscall.TimespecToNsec(nativeStat.Mtimespec)),
		Ctim:    NsecToTimespec(syscall.TimespecToNsec(nativeStat.Ctimespec)),
	}
}

func StatToSys(stat *Stat) any {
	return &syscall.Stat_t{
		Dev:     int32(stat.Dev),
		Ino:     stat.Ino,
		Nlink:   uint16(stat.Nlink),
		Mode:    uint16(stat.Mode),
		Uid:     stat.Uid,
		Gid:     stat.Gid,
		Rdev:    int32(stat.Rdev),
		Size:    stat.Size,
		Blksize: int32(stat.Blksize),
		Blocks:  stat.Blocks,
		Atimespec: syscall.Timespec{
			Sec:  stat.Atim.Sec,
			Nsec: stat.Atim.Nsec,
		},
		Mtimespec: syscall.Timespec{
			Sec:  stat.Mtim.Sec,
			Nsec: stat.Mtim.Nsec,
		},
		Ctimespec: syscall.Timespec{
			Sec:  stat.Ctim.Sec,
			Nsec: stat.Ctim.Nsec,
		},
	}
}
