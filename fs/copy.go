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
	srcInfo, srcErr := Stat(srcFS, srcPath)
	if srcErr != nil {
		return srcErr
	}
	dstInfo, dstErr := Stat(dstFS, dstPath)
	if dstErr == nil && !dstInfo.IsDir() {
		return fmt.Errorf("will not overwrite %q", dstPath)
	}
	switch mode := srcInfo.Mode(); mode & ModeType {
	case os.ModeSymlink:
		return copySymlink(srcFS, srcPath, dstFS, dstPath)
	case os.ModeDir:
		return copyDir(srcFS, srcPath, dstFS, dstPath, mode)
	case 0:
		return copyFile(srcFS, srcPath, dstFS, dstPath, mode)
	default:
		return fmt.Errorf("cannot copy file with mode %v", mode)
	}
}

func copySymlink(srcFS FS, srcPath string, dstFS FS, dstPath string) error {
	target, err := Readlink(srcFS, srcPath)
	if err != nil {
		return err
	}
	return Symlink(dstFS, target, dstPath)
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
		dstf.Close()
	}()
	defer func() {
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
	if dstPath == "." {
		return nil
	}
	if err := Chmod(dstFS, dstPath, mode.Perm()); err != nil {
		return err
	}
	return nil
}
