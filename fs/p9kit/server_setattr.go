package p9kit

import (
	"errors"
	"log"
	"time"

	"github.com/hugelgupf/p9/linux"
	"github.com/hugelgupf/p9/p9"
	"tractor.dev/wanix/fs"
)

// SetAttr implements p9.File.SetAttr.
func (l *p9file) SetAttr(valid p9.SetAttrMask, attr p9.SetAttr) error {
	// Define what attributes we support
	supported := p9.SetAttrMask{
		Size:               true,
		MTime:              true,
		CTime:              true,
		ATime:              true,
		MTimeNotSystemTime: true,
		ATimeNotSystemTime: true,
		Permissions:        true,
		UID:                true,
		GID:                true,
	}

	if !valid.IsSubsetOf(supported) {
		log.Printf("p9kit: unsupported attr: %v", valid)
		return linux.ENOSYS
	}

	// Handle size changes (truncate)
	if valid.Size {
		if err := fs.Truncate(l.fsys, l.path, int64(attr.Size)); err != nil {
			log.Printf("p9kit: truncate on %T: %s %s\n", l.fsys, l.path, err)
			return err
		}
	}

	// Handle time changes
	if valid.MTime || valid.ATime {
		var atime, mtime time.Time
		var needsUpdate bool

		// Get current file times as defaults
		if fi, err := fs.Stat(l.fsys, l.path); err == nil {
			atime = fi.ModTime() // Use mtime as fallback for atime
			mtime = fi.ModTime()
		} else {
			// Fallback to current time if we can't get file stats
			now := time.Now()
			atime = now
			mtime = now
		}

		// Handle access time
		if valid.ATime {
			needsUpdate = true
			if valid.ATimeNotSystemTime {
				// Use provided timestamp
				atime = time.Unix(int64(attr.ATimeSeconds), int64(attr.ATimeNanoSeconds))
			} else {
				// Use current system time
				atime = time.Now()
			}
		}

		// Handle modification time
		if valid.MTime {
			needsUpdate = true
			if valid.MTimeNotSystemTime {
				// Use provided timestamp
				mtime = time.Unix(int64(attr.MTimeSeconds), int64(attr.MTimeNanoSeconds))
			} else {
				// Use current system time
				mtime = time.Now()
			}
		}

		// Apply time changes if needed
		if needsUpdate {
			if err := fs.Chtimes(l.fsys, l.path, atime, mtime); err != nil {
				if errors.Is(err, fs.ErrNotSupported) {
					log.Printf("p9kit: chtimes on %T: %s %s\n", l.fsys, l.path, err)
				}
				return err
			}
		}
	}

	// Handle permission changes
	if valid.Permissions {
		if err := fs.Chmod(l.fsys, l.path, fs.FileMode(attr.Permissions)); err != nil {
			if errors.Is(err, fs.ErrNotSupported) {
				log.Printf("p9kit: chmod on %T: %s %s\n", l.fsys, l.path, err)
			}
			log.Println("p9kit: chmod on", l.path, attr.Permissions, err)
			return err
		}
	}

	// Handle ownership changes
	if valid.UID || valid.GID {
		// If virtual attributes are enabled, store UID/GID changes virtually
		if l.vattrs != nil {
			vattrs, err := l.vattrs.Get(l.path)
			if err != nil || vattrs == nil {
				vattrs = &VirtualAttrs{}
			}

			if valid.UID {
				uid := uint32(attr.UID)
				vattrs.UID = &uid
			}
			if valid.GID {
				gid := uint32(attr.GID)
				vattrs.GID = &gid
			}

			if err := l.vattrs.Set(l.path, vattrs); err != nil {
				return err
			}
		} else {
			// Fallback to actual filesystem changes if no virtual store
			uid := int(attr.UID)
			gid := int(attr.GID)

			// If only one is being changed, use -1 for the unchanged one
			// (standard Unix convention for chown)
			if !valid.UID {
				uid = -1
			}
			if !valid.GID {
				gid = -1
			}

			if err := fs.Chown(l.fsys, l.path, uid, gid); err != nil {
				if errors.Is(err, fs.ErrNotSupported) {
					log.Printf("p9kit: chown on %T: %s %s\n", l.fsys, l.path, err)
				}
				log.Println("p9kit: chown on", l.path, uid, gid, err)
				return err
			}
		}
	}

	return nil
}
