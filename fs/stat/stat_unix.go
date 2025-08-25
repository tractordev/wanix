//go:build !windows && !js && !wasip1
// +build !windows,!js,!wasip1

package stat

import (
	"golang.org/x/sys/unix"
)

// Stat_t is the Linux Stat_t.
type Stat_t = unix.Stat_t
