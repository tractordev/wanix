package fs

import (
	"fmt"
	"io"
	"os"
)

func IsDir(fsys FS, path string) (bool, error) {
	fi, err := Stat(fsys, path)
	if err != nil {
		return false, err
	}
	return fi.IsDir(), nil
}

func IsEmpty(fsys FS, path string) (bool, error) {
	if b, _ := Exists(fsys, path); !b {
		return false, fmt.Errorf("path does not exist: %q", path)
	}
	fi, err := Stat(fsys, path)
	if err != nil {
		return false, err
	}
	if fi.IsDir() {
		f, err := fsys.Open(path)
		if err != nil {
			return false, err
		}
		defer f.Close()
		list, _ := ReadDir(fsys, path)
		return len(list) == 0, nil
	}
	return fi.Size() == 0, nil
}

func Exists(fsys FS, path string) (bool, error) {
	_, err := Stat(fsys, path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func DirExists(fsys FS, path string) (bool, error) {
	fi, err := Stat(fsys, path)
	if err == nil && fi.IsDir() {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func WriteFile(fsys FS, filename string, data []byte, perm FileMode) error {
	// TODO: use Create, which should fallback to OpenFile
	of, ok := fsys.(OpenFileFS)
	if !ok {
		return ErrPermission
	}
	f, err := of.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	fw, ok := f.(io.WriteCloser)
	if !ok {
		f.Close()
		return ErrPermission
	}
	n, err := fw.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	if err1 := fw.Close(); err == nil {
		err = err1
	}
	return err
}
