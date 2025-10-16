package fs

import (
	"fmt"
	"io"
	"os"
	"reflect"
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

func IsSymlink(mode FileMode) bool {
	return mode&ModeSymlink != 0
}

func Exists(fsys FS, path string) (bool, error) {
	_, err := Lstat(fsys, path)
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

func AppendFile(fsys FS, filename string, data []byte) error {
	f, err := fsys.Open(filename)
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	n, err := WriteAt(f, data, fi.Size())
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err
}

func Equal(a, b FS) bool {
	return reflect.DeepEqual(a, b)
}
