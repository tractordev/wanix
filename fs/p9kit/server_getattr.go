package p9kit

import (
	"os"
	"time"

	"github.com/hugelgupf/p9/linux"
	"github.com/hugelgupf/p9/p9"
	"tractor.dev/wanix/fs/pstat"
)

var startTime = time.Now()

// GetAttr implements p9.File.GetAttr.
func (l *p9file) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	qid, fi, err := l.info()
	if err != nil {
		if os.IsNotExist(err) {
			return qid, p9.AttrMask{}, p9.Attr{}, linux.ENOENT
		}
		if os.IsPermission(err) {
			return qid, p9.AttrMask{}, p9.Attr{}, linux.EPERM
		}
		return qid, p9.AttrMask{}, p9.Attr{}, err
	}

	m := p9.ModeFromOS(fi.Mode())

	// Get system-specific info if available
	var (
		uid, gid     int
		nlink        uint64 = 1
		rdev         uint64 = 0
		blockSize    uint64 = 65536                               // reasonable default
		blocks       uint64 = uint64((fi.Size() + 65535) / 65536) // rough estimation
		atimeSeconds uint64
		atimeNano    uint64
		ctimeSeconds uint64
		ctimeNano    uint64
	)

	if st := pstat.FileInfoToStat(fi); st != nil {
		if l.vattrs == nil {
			uid = int(st.Uid)
			gid = int(st.Gid)
		}
		nlink = uint64(st.Nlink)
		rdev = uint64(st.Rdev)
		blockSize = uint64(st.Blksize)
		blocks = uint64(st.Blocks)
		// Use ModTime for all timestamps since js/wasm doesn't provide access times
		now := time.Now()
		atimeSeconds = uint64(now.Unix())
		atimeNano = uint64(now.Nanosecond())
		ctimeSeconds = uint64(fi.ModTime().Unix())
		ctimeNano = uint64(fi.ModTime().Nanosecond())
	} else {
		// Fallback to reasonable defaults if system-specific info is not available
		atimeSeconds = uint64(fi.ModTime().Unix())
		atimeNano = uint64(fi.ModTime().Nanosecond())
		ctimeSeconds = uint64(startTime.Unix())
		ctimeNano = uint64(startTime.Nanosecond())
	}

	// Build attribute response based on request mask
	attr := &p9.Attr{
		Mode:             m,
		UID:              p9.UID(uid),
		GID:              p9.GID(gid),
		NLink:            p9.NLink(nlink),
		RDev:             p9.Dev(rdev),
		Size:             uint64(fi.Size()),
		BlockSize:        blockSize,
		Blocks:           blocks,
		ATimeSeconds:     atimeSeconds,
		ATimeNanoSeconds: atimeNano,
		MTimeSeconds:     uint64(fi.ModTime().Unix()),
		MTimeNanoSeconds: uint64(fi.ModTime().Nanosecond()),
		CTimeSeconds:     ctimeSeconds,
		CTimeNanoSeconds: ctimeNano,
	}

	// Apply virtual attributes if available
	if l.vattrs != nil {
		if vattrs, err := l.vattrs.Get(l.path); err == nil {
			if vattrs != nil {
				if vattrs.UID != nil {
					attr.UID = p9.UID(*vattrs.UID)
				}
				if vattrs.GID != nil {
					attr.GID = p9.GID(*vattrs.GID)
				}
			}
		}
	}

	return qid, req, *attr, nil
}
