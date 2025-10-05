//go:build js || wasip1 || windows

package pstat

func SysToStat(sys any) *Stat {
	return &Stat{}
}

func StatToSys(stat *Stat) any {
	return stat
}
