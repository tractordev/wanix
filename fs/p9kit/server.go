package p9kit

import (
	"context"
	"errors"
	"hash/fnv"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/hugelgupf/p9/fsimpl/templatefs"
	"github.com/hugelgupf/p9/linux"
	"github.com/hugelgupf/p9/p9"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/pstat"
)

// AttacherOption interface for configuring the attacher
type AttacherOption interface {
	applyToAttacher(*attacher)
}

type attacher struct {
	fs.FS
	vattrs VirtualAttrStore // nil means no virtual attributes
}

var (
	_ p9.Attacher = &attacher{}
)

// Handle AT_REMOVEDIR (0x200) flag
const AT_REMOVEDIR = 0x200

func Attacher(fsys fs.FS, options ...AttacherOption) p9.Attacher {
	a := &attacher{FS: fsys}

	for _, opt := range options {
		opt.applyToAttacher(a)
	}

	return a
}

// Attach implements p9.Attacher.Attach.
func (a *attacher) Attach() (p9.File, error) {
	return &p9file{path: ".", fsys: a.FS, vattrs: a.vattrs}, nil
}

func toQid(name string, _ fs.FileInfo) (uint64, error) {
	h := fnv.New64a() // FNV-1a 64-bit hash
	h.Write([]byte(name))
	return h.Sum64(), nil
}

type p9file struct {
	templatefs.NotImplementedFile

	path      string
	file      fs.File
	fsys      fs.FS
	openFlags p9.OpenFlags
	vattrs    VirtualAttrStore // nil means no virtual attributes
}

var (
	// p9file is a p9.File
	_ p9.File = &p9file{}
)

// info constructs a QID for this file.
func (l *p9file) info() (p9.QID, fs.FileInfo, error) {
	var (
		qid p9.QID
		fi  fs.FileInfo
		err error
	)

	// Stat the file.
	if l.file != nil {
		fi, err = l.file.Stat()
	} else {
		ctx := context.Background()
		fi, err = fs.StatContext(fs.WithNoFollow(ctx), l.fsys, l.path)
		if err != nil {
			return qid, nil, err
		}
	}
	if err != nil {
		return qid, nil, err
	}

	// Construct the QID type.
	qid.Type = p9.ModeFromOS(fi.Mode()).QIDType()

	// Save the path from the Ino.
	ninePath, err := toQid(l.path, fi)
	if err != nil {
		return qid, nil, err
	}

	qid.Path = ninePath

	// this prevents caching on
	// linux when mounted with fscache
	// TODO: determine "static" files and give version
	qid.Version = 0

	return qid, fi, nil
}

// Walk implements p9.File.Walk.
func (l *p9file) Walk(names []string) ([]p9.QID, p9.File, error) {
	// log.Println("server walk:", l.path, names)
	var qids []p9.QID
	last := &p9file{path: l.path, fsys: l.fsys, vattrs: l.vattrs}

	// A walk with no names is a copy of self.
	if len(names) == 0 {
		return nil, last, nil
	}

	for _, name := range names {
		c := &p9file{path: path.Join(last.path, name), fsys: l.fsys, vattrs: l.vattrs}
		qid, _, err := c.info()
		if err != nil {
			return nil, nil, err
		}
		qids = append(qids, qid)
		last = c
	}
	return qids, last, nil
}

// FSync implements p9.File.FSync.
func (l *p9file) FSync() error {
	err := fs.Sync(l.file)
	if errors.Is(err, fs.ErrNotSupported) {
		return nil
	}
	return err
}

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
		blockSize    uint64 = 4096                              // reasonable default
		blocks       uint64 = uint64((fi.Size() + 4095) / 4096) // rough estimation
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

// Close implements p9.File.Close.
func (l *p9file) Close() error {
	if l.file != nil {
		// We don't set l.file = nil, as Close is called by servers
		// only in Clunk. Clunk should release the last (direct)
		// reference to this file.
		return l.file.Close()
	}
	return nil
}

// Open implements p9.File.Open.
func (l *p9file) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	// log.Println("server open:", l.path)
	qid, _, err := l.info()
	if err != nil {
		return qid, 0, err
	}

	// Use 0666 for new files (umask will be applied by the OS)
	// This matches standard Unix behavior
	perm := fs.FileMode(0666)

	f, err := fs.OpenFile(l.fsys, l.path, mode.OSFlags(), perm.Perm())
	if err != nil {
		return qid, 0, err
	}
	l.file = f
	l.openFlags = mode // Store the open flags

	return qid, 0, nil
}

// ReadAt implements p9.File.ReadAt.
func (l *p9file) ReadAt(p []byte, offset int64) (int, error) {
	return fs.ReadAt(l.file, p, offset)
}

// StatFS implements p9.File.StatFS.
func (l *p9file) StatFS() (p9.FSStat, error) {
	return p9.FSStat{}, nil
}

// Lock implements p9.File.Lock.
func (l *p9file) Lock(pid int, locktype p9.LockType, flags p9.LockFlags, start, length uint64, client string) (p9.LockStatus, error) {
	// TODO: implement?
	return p9.LockStatusOK, nil
}

// WriteAt implements p9.File.WriteAt.
func (l *p9file) WriteAt(p []byte, offset int64) (int, error) {
	// Check if file was opened with O_APPEND
	if l.openFlags&p9.OpenFlags(p9.Append) != 0 {
		// For append mode, ignore offset and write at end
		return fs.Write(l.file, p)
	}

	i, err := fs.WriteAt(l.file, p, offset)
	if err != nil && (errors.Is(err, fs.ErrNotSupported) || strings.Contains(err.Error(), "O_APPEND")) {
		log.Println(err)
	}
	return i, err
}

// Create implements p9.File.Create.
func (l *p9file) Create(name string, mode p9.OpenFlags, permissions p9.FileMode, _ p9.UID, _ p9.GID) (p9.File, p9.QID, uint32, error) {
	newName := path.Join(l.path, name)
	f, err := fs.OpenFile(l.fsys, newName, mode.OSFlags()|os.O_CREATE, permissions.OSMode())
	if err != nil {
		return nil, p9.QID{}, 0, err
	}

	l2 := &p9file{path: newName, file: f, fsys: l.fsys, vattrs: l.vattrs, openFlags: mode}
	qid, _, err := l2.info()
	if err != nil {
		l2.Close()
		return nil, p9.QID{}, 0, err
	}
	return l2, qid, 0, nil
}

// Mkdir implements p9.File.Mkdir.
func (l *p9file) Mkdir(name string, permissions p9.FileMode, _ p9.UID, _ p9.GID) (p9.QID, error) {
	if err := fs.Mkdir(l.fsys, path.Join(l.path, name), permissions.OSMode()); err != nil {
		return p9.QID{}, err
	}

	// Blank QID.
	return p9.QID{}, nil
}

// Symlink implements p9.File.Symlink.
func (l *p9file) Symlink(oldname string, newname string, _ p9.UID, _ p9.GID) (p9.QID, error) {
	if err := fs.Symlink(l.fsys, oldname, path.Join(l.path, newname)); err != nil {
		//log.Println("p9kit:", err, oldname, path.Join(l.path, newname))
		return p9.QID{}, err
	}

	// Blank QID.
	return p9.QID{}, nil
}

// Link implements p9.File.Link.
//
// Not properly implemented.
// func (l *p9file) Link(target p9.File, newname string) error {
// 	return os.Link(target.(*p9file).path, path.Join(l.path, newname))
// }

// RenameAt implements p9.File.RenameAt.
func (l *p9file) RenameAt(oldName string, newDir p9.File, newName string) error {
	oldPath := path.Join(l.path, oldName)
	newPath := path.Join(newDir.(*p9file).path, newName)

	return fs.Rename(l.fsys, oldPath, newPath)
}

// Readlink implements p9.File.Readlink.
func (l *p9file) Readlink() (string, error) {
	return fs.Readlink(l.fsys, l.path)
}

// Renamed implements p9.File.Renamed.
func (l *p9file) Renamed(parent p9.File, newName string) {
	l.path = path.Join(parent.(*p9file).path, newName)
}

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
			if errors.Is(err, fs.ErrNotSupported) {
				log.Printf("p9kit: truncate on %T: %s %s\n", l.fsys, l.path, err)
			}
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

// UnlinkAt implements p9.File.UnlinkAt
func (l *p9file) UnlinkAt(name string, flags uint32) error {
	// Construct the full path
	fullPath := filepath.Join(l.path, name)

	// Check if the target is a directory
	info, err := fs.StatContext(fs.WithNoFollow(context.Background()), l.fsys, fullPath)
	if err != nil {
		return err
	}

	// If target is a directory but AT_REMOVEDIR flag is not set, return EISDIR
	if info.IsDir() && (flags&AT_REMOVEDIR) == 0 {
		return linux.EISDIR
	}

	// If target is not a directory but AT_REMOVEDIR flag is set, return ENOTDIR
	if !info.IsDir() && (flags&AT_REMOVEDIR) != 0 {
		return linux.ENOTDIR
	}

	if info.IsDir() {
		children, err := fs.ReadDir(l.fsys, fullPath)
		if err != nil {
			return err
		}
		if len(children) > 0 {
			return linux.ENOTEMPTY
		}
	}

	// Remove the file or directory
	return fs.Remove(l.fsys, fullPath)
}

// Readdir implements p9.File.Readdir.
func (l *p9file) Readdir(offset uint64, count uint32) (dents p9.Dirents, derr error) {
	// log.Println("p9kit: readdir start:", l.path, "offset:", offset, "count:", count)
	// defer func() {
	// 	log.Println("p9kit: readdir end:", l.path, "returned:", len(dents), "entries, err:", derr)
	// }()

	var (
		p9Ents    = make([]p9.Dirent, 0)
		cursor    = uint64(0)
		seenNames = make(map[string]bool) // Track seen entry names to detect duplicates
	)

	entries, err := fs.ReadDir(l.fsys, l.path)
	if err != nil {
		return nil, err
	}

	// Use the entries already fetched from fs.ReadDir above
	for _, e := range entries {
		cursor++

		// Skip entries before the requested offset
		if cursor <= offset {
			continue
		}

		// Stop if we've gone past the requested range or filled the output slice
		if len(p9Ents) >= int(count) || cursor > offset+uint64(count) {
			break
		}

		// Detect duplicate entries to prevent infinite loops
		entryName := e.Name()
		if seenNames[entryName] {
			log.Printf("p9kit: detected duplicate entry %q at cursor %d, breaking to prevent infinite loop", entryName, cursor)
			break
		}
		seenNames[entryName] = true

		localEnt := p9file{path: path.Join(l.path, e.Name()), fsys: l.fsys, vattrs: l.vattrs}
		qid, _, err := localEnt.info()
		if err != nil {
			return p9Ents, err
		}

		p9Ents = append(p9Ents, p9.Dirent{
			QID:    qid,
			Type:   qidTypeToDirentType(qid.Type), // holy hell
			Name:   e.Name(),
			Offset: cursor,
		})
	}

	return p9Ents, nil
}

func qidTypeToDirentType(qtype p9.QIDType) p9.QIDType {
	if qtype&p9.TypeDir != 0 {
		return p9.QIDType(4) // DT_DIR
	}
	return p9.QIDType(8) // DT_REG
}

func (l *p9file) SetXattr(attr string, data []byte, flags p9.XattrFlags) error {
	ctx := fs.WithNoFollow(context.Background())
	return fs.SetXattr(ctx, l.fsys, l.path, attr, data, int(flags))
}

func (l *p9file) GetXattr(attr string) ([]byte, error) {
	ctx := fs.WithNoFollow(context.Background())
	return fs.GetXattr(ctx, l.fsys, l.path, attr)
}

func (l *p9file) ListXattrs() ([]string, error) {
	ctx := fs.WithNoFollow(context.Background())
	return fs.ListXattrs(ctx, l.fsys, l.path)
}

func (l *p9file) RemoveXattr(attr string) error {
	ctx := fs.WithNoFollow(context.Background())
	return fs.RemoveXattr(ctx, l.fsys, l.path, attr)
}
