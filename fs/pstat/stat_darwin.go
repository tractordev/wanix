//go:build linux || dragonfly || solaris
// +build linux dragonfly solaris

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
		Dev:     nativeStat.Dev,
		Ino:     nativeStat.Ino,
		Nlink:   nativeStat.Nlink,
		Mode:    nativeStat.Mode,
		Uid:     nativeStat.Uid,
		Gid:     nativeStat.Gid,
		Rdev:    nativeStat.Rdev,
		Size:    nativeStat.Size,
		Blksize: nativeStat.Blksize,
		Blocks:  nativeStat.Blocks,
		Atim:    NsecToTimespec(syscall.TimespecToNsec(nativeStat.Atim)),
		Mtim:    NsecToTimespec(syscall.TimespecToNsec(nativeStat.Mtim)),
		Ctim:    NsecToTimespec(syscall.TimespecToNsec(nativeStat.Ctim)),
	}
}

func StatToSys(stat *Stat) any {
	return &syscall.Stat_t{
		Dev:     stat.Dev,
		Ino:     stat.Ino,
		Nlink:   stat.Nlink,
		Mode:    stat.Mode,
		Uid:     stat.Uid,
		Gid:     stat.Gid,
		Rdev:    stat.Rdev,
		Size:    stat.Size,
		Blksize: stat.Blksize,
		Blocks:  stat.Blocks,
		Atim:    syscall.NsecToTimespec(stat.Atim.Nsec),
		Mtim:    syscall.NsecToTimespec(stat.Mtim.Nsec),
		Ctim:    syscall.NsecToTimespec(stat.Ctim.Nsec),
	}
}
