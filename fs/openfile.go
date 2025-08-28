package fs

import (
	"os"
)

type OpenFileFS interface {
	FS
	OpenFile(name string, flag int, perm FileMode) (File, error)
}

// OpenFile is a helper that opens a file with the given flag and permissions if supported.
func OpenFile(fsys FS, name string, flag int, perm FileMode) (File, error) {
	if o, ok := fsys.(OpenFileFS); ok {
		return o.OpenFile(name, flag, perm)
	}

	ctx := ContextFor(fsys)
	if flag&os.O_RDONLY != 0 {
		ctx = WithReadOnly(ctx)
	}

	rfsys, rname, err := ResolveTo[OpenFileFS](fsys, ctx, name)
	if err == nil {
		return rfsys.OpenFile(rname, flag, perm)
	}

	// Log all open flags
	// log.Println(name, fsutil.ParseOpenFlags(flag), fsutil.ParseFileMode(perm))

	// Handle write-only and read-write modes
	if flag&os.O_WRONLY != 0 || flag&os.O_RDWR != 0 {
		// O_CREATE means create file if it doesn't exist
		if flag&os.O_CREATE != 0 {
			f, err := fsys.Open(name)
			if err != nil {
				if os.IsNotExist(err) {
					f, err := Create(fsys, name)
					if err != nil {
						return nil, err
					}
					if perm != 0 {
						if err := Chmod(fsys, name, perm); err != nil {
							f.Close()
							return nil, err
						}
					}
					return f, nil
				}
				return nil, err
			}
			// O_TRUNC means truncate existing file
			if flag&os.O_TRUNC != 0 {
				// Close and recreate to truncate
				f.Close()
				f, err := Create(fsys, name)
				if err != nil {
					return nil, err
				}
				if perm != 0 {
					if err := Chmod(fsys, name, perm); err != nil {
						f.Close()
						return nil, err
					}
				}
				return f, nil
			}
			return f, nil
		}
		// No O_CREATE - file must exist
		return fsys.Open(name)
	}

	return fsys.Open(name)
}
