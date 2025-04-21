package p9kit

import (
	"errors"
	"hash/fnv"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/hugelgupf/p9/fsimpl/templatefs"
	"github.com/hugelgupf/p9/linux"
	"github.com/hugelgupf/p9/p9"
	"tractor.dev/wanix/fs"
)

type attacher struct {
	fs.FS
}

var (
	_ p9.Attacher = &attacher{}
)

func Attacher(fsys fs.FS) p9.Attacher {
	return &attacher{fsys} //fskit.NamedFS(fsys, "root")}
}

// Attach implements p9.Attacher.Attach.
func (a *attacher) Attach() (p9.File, error) {
	return &p9file{path: ".", fsys: a.FS}, nil
}

func toQid(name string, _ fs.FileInfo) (uint64, error) {
	h := fnv.New64a() // FNV-1a 64-bit hash
	h.Write([]byte(name))
	return h.Sum64(), nil
}

type p9file struct {
	templatefs.NotImplementedFile

	path string
	file fs.File
	fsys fs.FS
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
	// if l.file != nil {
	// 	fi, err = l.file.Stat()
	// } else {
	//	fi, err = fs.Stat(l.fsys, l.path)
	// }

	// prefer fs.Stat since symlinks are resolved
	fi, err = fs.Stat(l.fsys, l.path)

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
	last := &p9file{path: l.path, fsys: l.fsys}

	// A walk with no names is a copy of self.
	if len(names) == 0 {
		return nil, last, nil
	}

	for _, name := range names {
		c := &p9file{path: path.Join(last.path, name), fsys: l.fsys}
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
//
// Not fully implemented.
func (l *p9file) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	qid, fi, err := l.info()
	if err != nil {
		return qid, p9.AttrMask{}, p9.Attr{}, err
	}

	var m p9.FileMode
	if fi.Mode().IsDir() {
		m = p9.ModeDirectory
	} else if fi.Mode().IsRegular() {
		m = p9.ModeRegular
	} else if fi.Mode()&fs.ModeSymlink != 0 {
		m = p9.ModeSymlink
	}
	m |= p9.FileMode(fi.Mode().Perm())

	attr := &p9.Attr{
		Mode: m,
		// UID:              p9.UID(stat.Uid),
		// GID:              p9.GID(stat.Gid),
		NLink: p9.NLink(1),
		// RDev:  p9.Dev(250 << 8),
		Size: uint64(fi.Size()),
		// BlockSize:        uint64(stat.Blksize),
		// Blocks:           uint64(stat.Blocks),
		MTimeSeconds:     uint64(fi.ModTime().Unix()),
		MTimeNanoSeconds: uint64(fi.ModTime().Nanosecond()),
		// ATimeSeconds:     uint64(stat.Atim.Sec),
		// ATimeNanoSeconds: uint64(stat.Atim.Nsec),
		CTimeSeconds:     uint64(startTime.Unix()),
		CTimeNanoSeconds: uint64(startTime.Nanosecond()),
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

	// Do the actual open.

	f, err := fs.OpenFile(l.fsys, l.path, int(mode), 0)
	if err != nil {
		return qid, 0, err
	}
	l.file = f

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
	i, err := fs.WriteAt(l.file, p, offset)
	if errors.Is(err, fs.ErrNotSupported) {
		log.Println(err)
	}
	return i, err
}

// Create implements p9.File.Create.
func (l *p9file) Create(name string, mode p9.OpenFlags, permissions p9.FileMode, _ p9.UID, _ p9.GID) (p9.File, p9.QID, uint32, error) {
	newName := path.Join(l.path, name)
	f, err := fs.OpenFile(l.fsys, newName, int(mode)|os.O_CREATE|os.O_EXCL, fs.FileMode(permissions))
	if err != nil {
		return nil, p9.QID{}, 0, err
	}

	l2 := &p9file{path: newName, file: f, fsys: l.fsys}
	qid, _, err := l2.info()
	if err != nil {
		l2.Close()
		return nil, p9.QID{}, 0, err
	}
	return l2, qid, 0, nil
}

// Mkdir implements p9.File.Mkdir.
//
// Not properly implemented.
func (l *p9file) Mkdir(name string, permissions p9.FileMode, _ p9.UID, _ p9.GID) (p9.QID, error) {
	if err := fs.Mkdir(l.fsys, path.Join(l.path, name), fs.FileMode(permissions)); err != nil {
		return p9.QID{}, err
	}

	// Blank QID.
	return p9.QID{}, nil
}

// Symlink implements p9.File.Symlink.
func (l *p9file) Symlink(oldname string, newname string, _ p9.UID, _ p9.GID) (p9.QID, error) {
	if err := fs.Symlink(l.fsys, oldname, path.Join(l.path, newname)); err != nil {
		log.Println("p9kit:", err, oldname, path.Join(l.path, newname))
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

	err := fs.Rename(l.fsys, oldPath, newPath)
	if err != nil {
		log.Println("RENAME:", err, oldPath, newPath)
	}
	return err
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
	// When truncate(2) is called on Linux, Linux will try to set time & size. Fake it. Sorry.
	supported := p9.SetAttrMask{Size: true, MTime: true, CTime: true, ATime: true}
	if !valid.IsSubsetOf(supported) {
		log.Printf("p9kit: unsupported attr: %v", valid)
		return linux.ENOSYS
	}

	if valid.Size {
		if err := fs.Truncate(l.fsys, l.path, int64(attr.Size)); err != nil {
			if errors.Is(err, fs.ErrNotSupported) {
				log.Printf("p9kit: truncate on %T: %s %s\n", l.fsys, l.path, err)
			}
			return err
		}
	}

	if valid.MTime || valid.ATime {
		if err := fs.Chtimes(l.fsys, l.path,
			time.Unix(int64(attr.ATimeSeconds), int64(attr.ATimeNanoSeconds)),
			time.Unix(int64(attr.MTimeSeconds), int64(attr.MTimeNanoSeconds))); err != nil {
			if errors.Is(err, fs.ErrNotSupported) {
				log.Printf("p9kit: chtimes on %T: %s %s\n", l.fsys, l.path, err)
			}
			return err
		}
	}

	return nil
}

// UnlinkAt implements p9.File.UnlinkAt
func (l *p9file) UnlinkAt(name string, flags uint32) error {
	// Construct the full path
	fullPath := filepath.Join(l.path, name)

	// Remove the file or directory
	return fs.Remove(l.fsys, fullPath)
}

// Readdir implements p9.File.Readdir.
func (l *p9file) Readdir(offset uint64, count uint32) (p9.Dirents, error) {
	// log.Println("server readdir:", l.path)
	var (
		p9Ents = make([]p9.Dirent, 0)
		cursor = uint64(0)
	)

	dirfile, ok := l.file.(fs.ReadDirFile)
	if !ok {
		return nil, linux.ENOTDIR
	}

	for len(p9Ents) < int(count) {
		singleEnt, err := dirfile.ReadDir(1)

		if err == io.EOF {
			return p9Ents, nil
		} else if err != nil {
			return nil, err
		}

		// we consumed an entry
		cursor++

		// cursor \in (offset, offset+count)
		if cursor < offset || cursor > offset+uint64(count) {
			continue
		}

		e := singleEnt[0]

		localEnt := p9file{path: path.Join(l.path, e.Name()), fsys: l.fsys}
		qid, _, err := localEnt.info()
		if err != nil {
			return p9Ents, err
		}
		p9Ents = append(p9Ents, p9.Dirent{
			QID:    qid,
			Type:   qid.Type,
			Name:   e.Name(),
			Offset: cursor,
		})
	}

	return p9Ents, nil
}
