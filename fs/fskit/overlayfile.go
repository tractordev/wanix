package fskit

import (
	"io"
	"os"
	"sort"

	"tractor.dev/wanix/fs"
)

type OverlayFile struct {
	Base    fs.File
	Overlay fs.File
	HideFn  func(name string) bool // if true, entry is hidden from readdir results
	iter    *DirIter
}

func (f *OverlayFile) Stat() (os.FileInfo, error) {
	if f.Overlay != nil {
		return f.Overlay.Stat()
	}
	if f.Base != nil {
		return f.Base.Stat()
	}
	return nil, fs.ErrInvalid
}

func (f *OverlayFile) Close() error {
	// first close base, so we have a newer timestamp in the overlay. If we'd close
	// the overlay first, we'd get a cacheStale the next time we access this file
	// -> cache would be useless ;-)
	if f.Base != nil {
		f.Base.Close()
	}
	if f.Overlay != nil {
		return f.Overlay.Close()
	}
	return fs.ErrInvalid
}

func (f *OverlayFile) ReadDir(c int) ([]fs.DirEntry, error) {
	if f.iter == nil {
		f.iter = NewDirIter(func() (entries []fs.DirEntry, err error) {
			entryMap := make(map[string]fs.DirEntry)

			if brd, ok := f.Base.(fs.ReadDirFile); ok {
				ents, err := brd.ReadDir(-1)
				if err != nil {
					return nil, err
				}
				for _, entry := range ents {
					entryMap[entry.Name()] = entry
				}
			}
			if brd, ok := f.Overlay.(fs.ReadDirFile); ok {
				ents, err := brd.ReadDir(-1)
				if err != nil {
					return nil, err
				}
				for _, entry := range ents {
					entryMap[entry.Name()] = entry
				}
			}
			for _, entry := range entryMap {
				// Skip entries that should be hidden
				if f.HideFn != nil && f.HideFn(entry.Name()) {
					continue
				}
				entries = append(entries, entry)
			}
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].Name() < entries[j].Name()
			})
			return entries, nil
		})
	}
	return f.iter.ReadDir(c)
}

func (f *OverlayFile) Read(s []byte) (int, error) {
	if f.Overlay != nil {
		n, err := f.Overlay.Read(s)
		if (err == nil || err == io.EOF) && f.Base != nil {
			// advance the file position also in the base file, the next
			// call may be a write at this position (or a seek with SEEK_CUR)
			if _, seekErr := fs.Seek(f.Base, int64(n), io.SeekCurrent); seekErr != nil {
				// only overwrite err in case the seek fails: we need to
				// report an eventual io.EOF to the caller
				err = seekErr
			}
		}
		return n, err
	}
	if f.Base != nil {
		return f.Base.Read(s)
	}
	return 0, fs.ErrInvalid
}

func (f *OverlayFile) ReadAt(s []byte, o int64) (int, error) {
	if f.Overlay != nil {
		n, err := fs.ReadAt(f.Overlay, s, o)
		if (err == nil || err == io.EOF) && f.Base != nil {
			_, err = fs.Seek(f.Base, o+int64(n), io.SeekStart)
		}
		return n, err
	}
	if f.Base != nil {
		return fs.ReadAt(f.Base, s, o)
	}
	return 0, fs.ErrInvalid
}

func (f *OverlayFile) Seek(o int64, w int) (pos int64, err error) {
	if f.Overlay != nil {
		pos, err = fs.Seek(f.Overlay, o, w)
		if (err == nil || err == io.EOF) && f.Base != nil {
			_, err = fs.Seek(f.Base, o, w)
		}
		return pos, err
	}
	if f.Base != nil {
		return fs.Seek(f.Base, o, w)
	}
	return 0, fs.ErrInvalid
}

func (f *OverlayFile) Write(s []byte) (n int, err error) {
	if f.Overlay != nil {
		n, err = fs.Write(f.Overlay, s)
		if err == nil &&
			f.Base != nil { // hmm, do we have fixed size files where a write may hit the EOF mark?
			_, err = fs.Write(f.Base, s)
		}
		return n, err
	}
	if f.Base != nil {
		return fs.Write(f.Base, s)
	}
	return 0, fs.ErrInvalid
}

func (f *OverlayFile) WriteAt(s []byte, o int64) (n int, err error) {
	if f.Overlay != nil {
		n, err = fs.WriteAt(f.Overlay, s, o)
		if err == nil && f.Base != nil {
			_, err = fs.WriteAt(f.Base, s, o)
		}
		return n, err
	}
	if f.Base != nil {
		return fs.WriteAt(f.Base, s, o)
	}
	return 0, fs.ErrInvalid
}

func (f *OverlayFile) Sync() (err error) {
	if f.Overlay != nil {
		err = fs.Sync(f.Overlay)
		if err == nil && f.Base != nil {
			err = fs.Sync(f.Base)
		}
		return err
	}
	if f.Base != nil {
		return fs.Sync(f.Base)
	}
	return fs.ErrInvalid
}
