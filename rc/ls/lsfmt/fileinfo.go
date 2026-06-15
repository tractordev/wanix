package lsfmt

import (
	"os"
	"time"
)

// FileInfo holds file metadata for ls rendering.
type FileInfo struct {
	Name          string
	Mode          os.FileMode
	Rdev          uint64
	UID, GID      uint32
	Size          int64
	MTime         time.Time
	SymlinkTarget string
}

// FromOSFileInfo converts os.FileInfo to lsfmt.FileInfo.
func FromOSFileInfo(path string, fi os.FileInfo) FileInfo {
	var link string
	if fi.Mode()&os.ModeType == os.ModeSymlink {
		if l, err := os.Readlink(path); err != nil {
			link = err.Error()
		} else {
			link = l
		}
	}
	return FileInfo{
		Name:          fi.Name(),
		Mode:          fi.Mode(),
		Size:          fi.Size(),
		MTime:         fi.ModTime(),
		SymlinkTarget: link,
	}
}
