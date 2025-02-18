package p9kit

import (
	"io"
	"net"
	"path"
	"strings"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"

	"github.com/hugelgupf/p9/p9"
)

func ClientFS(conn net.Conn, aname string) (fs.FS, error) {
	client, err := p9.NewClient(conn)
	if err != nil {
		return nil, err
	}

	var root p9.File
	root, err = client.Attach(aname)
	if err != nil {
		return nil, err
	}

	return &FS{client: client, root: root}, nil
}

type FS struct {
	client *p9.Client
	root   p9.File
}

func walkParts(name string) (parts []string) {
	if name == "." {
		name = ""
	}
	if len(name) > 0 {
		parts = strings.Split(name, "/")
	}
	return
}

func fixErr(err error) error {
	if err == nil {
		return nil
	}
	if err.Error() == "file exists" {
		return fs.ErrExist
	}
	return err
}

func (fsys *FS) walk(name string) (p9.File, error) {
	_, f, err := fsys.root.Walk(walkParts(name))
	// log.Println("walk:", name, walkParts(name), err)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (fsys *FS) Create(name string) (fs.File, error) {
	if name == "." {
		return nil, fs.ErrInvalid
	}

	d, err := fsys.walk(path.Dir(name))
	if err != nil {
		return nil, err
	}

	f, _, _, err := d.Create(path.Base(name), p9.ReadWrite, p9.FileMode(0644), 0, 0)
	if err != nil {
		if fixErr(err) == fs.ErrExist {
			f, err = fsys.walk(name)
			if err != nil {
				return nil, err
			}
			if err := f.SetAttr(p9.SetAttrMask{Size: true}, p9.SetAttr{Size: 0}); err != nil {
				return nil, err
			}
			_, _, err = f.Open(p9.ReadWrite)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}

	}

	return &remoteFile{
		file: f,
		root: fsys.root,
		name: path.Base(name),
		path: walkParts(name),
	}, nil
}

func (fsys *FS) Open(name string) (fs.File, error) {
	f, err := fsys.walk(name)
	if err != nil {
		return nil, err
	}

	_, _, err = f.Open(p9.ReadWrite)
	if err != nil {
		// log.Printf("open: %s %v %T %d\n", name, err, err, uintptr(err.(linux.Errno)))
		// might be a dir and needs to be read-only
		_, _, err = f.Open(p9.ReadOnly)
		if err != nil {
			// log.Println("open2:", name, err)
			return nil, fixErr(err)
		}
	}

	return &remoteFile{
		file: f,
		root: fsys.root,
		name: path.Base(name),
		path: walkParts(name),
	}, nil
}

func (fsys *FS) Mkdir(name string, perm fs.FileMode) error {
	d, err := fsys.walk(path.Dir(name))
	if err != nil {
		return err
	}

	_, err = d.Mkdir(path.Base(name), p9.FileMode(perm), 0, 0)
	return fixErr(err)
}

func (fsys *FS) Remove(name string) error {
	d, err := fsys.walk(path.Dir(name))
	if err != nil {
		return err
	}

	err = d.UnlinkAt(path.Base(name), 0)
	return fixErr(err)
}

func (fsys *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	// log.Println("readdir:", name)
	f, err := fsys.walk(name)
	if err != nil {
		return nil, err
	}

	_, _, err = f.Open(p9.ReadOnly)
	if err != nil {
		// log.Println("readdir2:", name, err)
		return nil, err
	}

	var dirents []p9.Dirent
	offset := uint64(0)
	nleft := -1
	for {
		d, err := f.Readdir(offset, uint32(nleft))
		// log.Println("readdir3:", name, err)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(d) == 0 {
			break
		}
		dirents = append(dirents, d...)
		nleft -= len(d)
		offset = d[len(d)-1].Offset
	}

	var entries []fs.DirEntry
	for _, entry := range dirents {
		var walkPath []string
		if name != "." {
			walkPath = append(walkParts(name), entry.Name)
		} else {
			walkPath = []string{entry.Name}
		}
		_, child, err := fsys.root.Walk(walkPath)
		if err != nil {
			return nil, err
		}
		if fi, err := fileInfo(child, entry.Name); err == nil {
			entries = append(entries, fi.(fs.DirEntry))
		}
	}
	return entries, nil
}

type remoteFile struct {
	name   string
	file   p9.File
	root   p9.File
	path   []string
	offset int64
}

func (f *remoteFile) Read(p []byte) (n int, err error) {
	n, err = f.file.ReadAt(p, f.offset)
	if err != nil {
		return n, err
	}
	f.offset += int64(n)
	return n, nil
}

func (f *remoteFile) Write(p []byte) (n int, err error) {
	n, err = f.file.WriteAt(p, f.offset)
	if err != nil {
		return n, err
	}
	f.offset += int64(n)
	return n, nil
}

func (f *remoteFile) Close() error {
	if err := f.file.FSync(); err != nil {
		return err
	}
	return f.file.Close()
}

func fileInfo(f p9.File, name string) (fs.FileInfo, error) {
	_, _, attr, err := f.GetAttr(p9.AttrMask{
		Mode: true,
		// NLink:       true,
		// UID:         true,
		// GID:         true,
		// RDev:        true,
		ATime: true,
		MTime: true,
		CTime: true,
		// INo:         true,
		Size: true,
		// Blocks:      true,
		// BTime:       true,
		// Gen:         true,
		// DataVersion: true,
	})
	if err != nil {
		return nil, err
	}
	var mode fs.FileMode = fs.FileMode(attr.Mode)
	if attr.Mode&p9.ModeDirectory != 0 {
		mode |= fs.ModeDir
	}
	return fskit.Entry(
		name,
		mode,
		int64(attr.Size),
		time.Unix(int64(attr.MTimeSeconds), 0),
	), nil
}

func (f *remoteFile) Stat() (fs.FileInfo, error) {
	return fileInfo(f.file, f.name)
}

// func (f *remoteFile) ReadDir(n int) ([]fs.DirEntry, error) {
// 	fmt.Println("readdir:", f.name)
// 	var dirents []p9.Dirent
// 	offset := uint64(0)
// 	nleft := n
// 	for {
// 		d, err := f.file.Readdir(offset, uint32(nleft))
// 		if err != nil {
// 			if err == io.EOF {
// 				break
// 			}
// 			return nil, err
// 		}
// 		if len(d) == 0 {
// 			break
// 		}
// 		dirents = append(dirents, d...)
// 		nleft -= len(d)
// 		offset = d[len(d)-1].Offset
// 	}

// 	var entries []fs.DirEntry
// 	for _, entry := range dirents {
// 		_, child, err := f.root.Walk(append(f.path, entry.Name))
// 		if err != nil {
// 			return nil, err
// 		}
// 		if fi, err := fileInfo(child, entry.Name); err == nil {
// 			entries = append(entries, fi.(fs.DirEntry))
// 		}
// 	}
// 	return entries, nil
// }
