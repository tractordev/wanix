package fs

import "os"

type OpenFileFS interface {
	FS
	OpenFile(name string, flag int, perm FileMode) (File, error)
}

// OpenFile is a helper that opens a file with the given flag and permissions if supported.
func OpenFile(fsys FS, name string, flag int, perm FileMode) (File, error) {
	if o, ok := fsys.(OpenFileFS); ok {
		return o.OpenFile(name, flag, perm)
	}

	rfsys, rname, err := ResolveAs[OpenFileFS](fsys, name)
	if err == nil {
		return rfsys.OpenFile(rname, flag, perm)
	}

	// Log all open flags
	// log.Printf("openfile flags: O_RDONLY=%v O_WRONLY=%v O_RDWR=%v O_APPEND=%v O_CREATE=%v O_EXCL=%v O_SYNC=%v O_TRUNC=%v",
	// 	flag&os.O_RDONLY != 0,
	// 	flag&os.O_WRONLY != 0,
	// 	flag&os.O_RDWR != 0,
	// 	flag&os.O_APPEND != 0,
	// 	flag&os.O_CREATE != 0,
	// 	flag&os.O_EXCL != 0,
	// 	flag&os.O_SYNC != 0,
	// 	flag&os.O_TRUNC != 0)

	// if create flag is set
	if flag&os.O_CREATE != 0 {
		if flag&os.O_APPEND == 0 {
			// if not append, create a new file
			return Create(fsys, name)
		} else {
			// if append, open the file
			f, err := fsys.Open(name)
			if err != nil {
				// if file doesn't exist, create it
				if os.IsNotExist(err) {
					return Create(fsys, name)
				}
				return nil, err
			}
			// todo: seek to the end?
			return f, nil
		}
	}

	// just fall back to Open
	return fsys.Open(name)
}
