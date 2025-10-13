package tarfs

import (
	"archive/tar"
	"io"
	"io/fs"
	"os"
)

func Archive(fsys fs.FS, tw *tar.Writer) error {
	return fs.WalkDir(fsys, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Update the name to maintain directory structure
		header.Name = path

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			header.Linkname = link
		}

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// If it's not a regular file, we're done
		if !info.Mode().IsRegular() {
			return nil
		}

		// Open and copy file contents
		f, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}
