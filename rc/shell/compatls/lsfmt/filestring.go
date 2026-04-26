package lsfmt

import (
	"fmt"
	"os"
	"strconv"

	humanize "github.com/dustin/go-humanize"
)

// LongStringer formats file info in ls -l style.
type LongStringer struct {
	Human bool
	Name  Stringer
}

// FileString implements Stringer.FileString.
func (ls LongStringer) FileString(fi FileInfo) string {
	var size string
	if ls.Human {
		size = humanize.Bytes(uint64(fi.Size))
	} else {
		size = strconv.FormatInt(fi.Size, 10)
	}

	s := fmt.Sprintf("%s\t%d\t%d\t%s\t%v\t%s",
		fi.Mode.String(),
		fi.UID,
		fi.GID,
		size,
		fi.MTime.Format("Jan _2 15:04"),
		ls.Name.FileString(fi))

	if fi.Mode&os.ModeType == os.ModeSymlink {
		s += fmt.Sprintf(" -> %v", fi.SymlinkTarget)
	}
	return s
}
