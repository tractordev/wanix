package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyAll recursively copies the file, directory or symbolic link at src
// to dst. The destination must not exist. Symbolic links are not
// followed.
//
// If the copy fails half way through, the destination might be left
// partially written.
func CopyAll(fsys FS, src, dst string) error {
	return CopyFS(fsys, src, fsys, dst)
}

func CopyFS(srcFS FS, srcPath string, dstFS FS, dstPath string) error {
	srcInfo, srcErr := Lstat(srcFS, srcPath)
	if srcErr != nil {
		return srcErr
	}
	dstInfo, dstErr := Lstat(dstFS, dstPath)
	if dstErr == nil && !dstInfo.IsDir() {
		return fmt.Errorf("will not overwrite %q", dstPath)
	}
	switch mode := srcInfo.Mode(); mode & ModeType {
	case os.ModeSymlink:
		return copySymlink(srcFS, srcPath, dstFS, dstPath, mode)
	case os.ModeDir:
		return copyDir(srcFS, srcPath, dstFS, dstPath, mode)
	case 0:
		return copyFile(srcFS, srcPath, dstFS, dstPath, mode)
	default:
		return fmt.Errorf("cannot copy file with mode %v", mode)
	}
}

// CopyNewFS copies from srcFS/srcPath to dstFS/dstPath, overwriting files if src is newer.
func CopyNewFS(srcFS FS, srcPath string, dstFS FS, dstPath string) error {
	srcInfo, srcErr := Lstat(srcFS, srcPath)
	if srcErr != nil {
		return srcErr
	}
	dstInfo, dstErr := Lstat(dstFS, dstPath)

	// If destination exists and is a file, check mod time
	if dstErr == nil && !dstInfo.IsDir() {
		// Only overwrite if src is newer
		if !srcInfo.ModTime().After(dstInfo.ModTime()) {
			// Destination is up to date or newer, skip copy
			return nil
		}
	}

	switch mode := srcInfo.Mode(); mode & ModeType {
	case os.ModeSymlink:
		return copySymlink(srcFS, srcPath, dstFS, dstPath, mode)
	case os.ModeDir:
		return copyDirNewer(srcFS, srcPath, dstFS, dstPath, mode)
	case 0:
		return copyFile(srcFS, srcPath, dstFS, dstPath, mode)
	default:
		return fmt.Errorf("cannot copy file with mode %v", mode)
	}
}

// copyDirNewer recursively copies a directory, only overwriting files if src is newer.
func copyDirNewer(srcFS FS, srcPath string, dstFS FS, dstPath string, mode FileMode) error {
	srcf, err := srcFS.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcf.Close()
	if mode&0500 == 0 {
		// The source directory doesn't have write permission,
		// so give the new directory write permission anyway
		// so that we have permission to create its contents.
		// We'll make the permissions match at the end.
	}
	// Create the destination directory if it doesn't exist
	if err := Mkdir(dstFS, dstPath, mode.Perm()); err != nil && !os.IsExist(err) {
		return err
	}
	// Read directory entries
	entries, err := ReadDir(srcFS, srcPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcEntry := filepath.Join(srcPath, entry.Name())
		dstEntry := filepath.Join(dstPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := copyDirNewer(srcFS, srcEntry, dstFS, dstEntry, info.Mode()); err != nil {
				return err
			}
		} else if info.Mode()&os.ModeSymlink != 0 {
			if err := copySymlink(srcFS, srcEntry, dstFS, dstEntry, info.Mode()); err != nil {
				return err
			}
		} else {
			// For files, only copy if src is newer
			dstInfo, dstErr := Stat(dstFS, dstEntry)
			if dstErr == nil && !info.ModTime().After(dstInfo.ModTime()) {
				continue // skip, dst is up to date
			}
			if err := copyFile(srcFS, srcEntry, dstFS, dstEntry, info.Mode()); err != nil {
				return err
			}
		}
	}
	// Ensure directory permissions match source?
	// if err := Chmod(dstFS, dstPath, mode.Perm()); err != nil {
	// 	return fmt.Errorf("chmod: %w", err)
	// }
	return nil
}

func copySymlink(srcFS FS, srcPath string, dstFS FS, dstPath string, mode FileMode) error {
	target, err := Readlink(srcFS, srcPath)
	if err != nil {
		return err
	}
	err = Symlink(dstFS, target, dstPath)
	if err != nil {
		return err
	}
	return Chmod(dstFS, dstPath, mode.Perm())
}

func copyFile(srcFS FS, srcPath string, dstFS FS, dstPath string, mode FileMode) (err error) {
	srcf, e := srcFS.Open(srcPath)
	if e != nil {
		return e
	}
	defer srcf.Close()
	dstf, e := OpenFile(dstFS, dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
	if e != nil {
		return e
	}
	defer func() {
		if e := dstf.Close(); e != nil {
			err = fmt.Errorf("close: %w", e)
		}
		// Make the actual permissions match the source permissions
		// even in the presence of umask.
		if e := Chmod(dstFS, dstPath, mode.Perm()); e != nil {
			err = fmt.Errorf("chmod: %w", e)
		}
	}()
	wdstf, ok := dstf.(io.Writer)
	if !ok {
		return fmt.Errorf("cannot copy %q to %q: dst not writable", srcPath, dstPath)
	}
	if _, e := io.Copy(wdstf, srcf); err != nil {
		return fmt.Errorf("cannot copy %q to %q: %v", srcPath, dstPath, e)
	}
	return
}

func copyDir(srcFS FS, srcPath string, dstFS FS, dstPath string, mode FileMode) error {
	srcf, err := srcFS.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcf.Close()
	if mode&0500 == 0 {
		// The source directory doesn't have write permission,
		// so give the new directory write permission anyway
		// so that we have permission to create its contents.
		// We'll make the permissions match at the end.
		mode |= 0500
	}
	if err := MkdirAll(dstFS, dstPath, mode.Perm()); err != nil {
		return err
	}
	entries, err := ReadDir(srcFS, srcPath)
	if err != nil {
		return fmt.Errorf("error reading directory %q: %v", srcPath, err)
	}
	for _, entry := range entries {
		if err := CopyFS(srcFS, filepath.Join(srcPath, entry.Name()), dstFS, filepath.Join(dstPath, entry.Name())); err != nil {
			return err
		}
	}
	if err := Chmod(dstFS, dstPath, mode.Perm()); err != nil {
		return err
	}
	return nil
}
