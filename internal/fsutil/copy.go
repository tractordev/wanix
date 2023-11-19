package fsutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"tractor.dev/toolkit-go/engine/fs"
)

// CopyAll recursively copies the file, directory or symbolic link at src
// to dst. The destination must not exist. Symbolic links are not
// followed.
//
// If the copy fails half way through, the destination might be left
// partially written.
func CopyAll(fsys fs.MutableFS, src, dst string) error {
	srcInfo, srcErr := fs.Stat(fsys, src)
	if srcErr != nil {
		return srcErr
	}
	dstInfo, dstErr := fs.Stat(fsys, dst)
	if dstErr == nil {
		fmt.Println(dst, dstInfo.Name())
		return fmt.Errorf("will not overwrite %q", dst)
	}
	if !os.IsNotExist(dstErr) {
		return dstErr
	}
	switch mode := srcInfo.Mode(); mode & fs.ModeType {
	// case os.ModeSymlink:
	// 	return copySymLink(src, dst)
	case os.ModeDir:
		return copyDir(fsys, src, dst, mode)
	case 0:
		return copyFile(fsys, src, dst, mode)
	default:
		return fmt.Errorf("cannot copy file with mode %v", mode)
	}
}

// func copySymLink(src, dst string) error {
// 	target, err := os.Readlink(src)
// 	if err != nil {
// 		return err
// 	}
// 	return os.Symlink(target, dst)
// }

func copyFile(fsys fs.MutableFS, src, dst string, mode fs.FileMode) error {
	srcf, err := fsys.Open(src)
	if err != nil {
		return err
	}
	defer srcf.Close()
	dstf, err := fsys.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer dstf.Close()
	// Make the actual permissions match the source permissions
	// even in the presence of umask.
	if err := fsys.Chmod(dst, mode.Perm()); err != nil {
		return err
	}
	wdstf, ok := dstf.(io.Writer)
	if !ok {
		return fmt.Errorf("cannot copy %q to %q: dst not writable", src, dst)
	}
	if _, err := io.Copy(wdstf, srcf); err != nil {
		return fmt.Errorf("cannot copy %q to %q: %v", src, dst, err)
	}
	return nil
}

func copyDir(fsys fs.MutableFS, src, dst string, mode fs.FileMode) error {
	srcf, err := fsys.Open(src)
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
	if err := fsys.Mkdir(dst, mode.Perm()); err != nil {
		return err
	}
	entries, err := fs.ReadDir(fsys, src)
	if err != nil {
		return fmt.Errorf("error reading directory %q: %v", src, err)
	}
	for _, entry := range entries {
		if err := CopyAll(fsys, filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
			return err
		}
	}
	if err := fsys.Chmod(dst, mode.Perm()); err != nil {
		return err
	}
	return nil
}
