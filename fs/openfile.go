package fs

import (
	"context"
	"errors"
	"io"
	"os"
)

type OpenFileFS interface {
	FS
	OpenFile(name string, flag int, perm FileMode) (File, error)
}

type OpenFileContextFS interface {
	FS
	OpenFileContext(ctx context.Context, name string, flag int, perm FileMode) (File, error)
}

// OpenFile is a helper that opens a file with the given flag and permissions if supported.
func OpenFile(fsys FS, name string, flag int, perm FileMode) (f File, err error) {
	ctx := WithOrigin(ContextFor(fsys), fsys, name, "open")
	if flag&os.O_RDONLY != 0 {
		ctx = WithReadOnly(ctx)
	}

	if o, ok := fsys.(OpenFileContextFS); ok {
		return o.OpenFileContext(ctx, name, flag, perm)
	}

	if rfsys, rname, err := ResolveTo[OpenFileContextFS](fsys, ctx, name); err == nil {
		return rfsys.OpenFileContext(ctx, rname, flag, perm)
	}

	if o, ok := fsys.(OpenFileFS); ok {
		return o.OpenFile(name, flag, perm)
	}

	rfsys, rname, err := ResolveTo[OpenFileFS](fsys, ctx, name)
	if err == nil {
		return rfsys.OpenFile(rname, flag, perm)
	}

	// Handle write-only and read-write modes
	if flag&(os.O_WRONLY|os.O_RDWR) != 0 {
		created := false
		f, err = fsys.Open(name)
		if err != nil {
			// O_CREATE means create file if it doesn't exist
			if flag&os.O_CREATE != 0 && os.IsNotExist(err) {
				created = true
				f, err = Create(fsys, name)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
		// O_TRUNC means truncate existing file
		// but if we created the file, we don't need to truncate
		if flag&os.O_TRUNC != 0 && !created {
			// Close and recreate to truncate
			f.Close()
			f, err = Create(fsys, name)
			if err != nil {
				return nil, err
			}
		}
		if perm != 0 {
			f.Close()
			// close and reopen after chmod
			// since close might clobber the chmod
			if err := Chmod(fsys, name, perm); err != nil && !errors.Is(err, ErrNotSupported) {
				return nil, err
			}
			f, err = fsys.Open(name)
			if err != nil {
				return nil, err
			}
		}
		// O_APPEND means append to existing file
		if flag&os.O_APPEND != 0 {
			_, err = Seek(f, 0, io.SeekEnd)
			if err != nil {
				return nil, err
			}
		}
		return f, nil
	}
	return fsys.Open(name)
}
