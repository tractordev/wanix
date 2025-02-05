package fs

import (
	"errors"
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
	// var f File
	// var err error
	// if c, ok := fsys.(CreateFS); ok {
	// 	f, err = c.Create(filename)
	// 	if err != nil {
	// 		return err
	// 	}
	// } else if of, ok := fsys.(OpenFileFS); ok {
	// 	f, err = of.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	// 	if err != nil {
	// 		return err
	// 	}
	// } else {
	// 	// for now, we'll fallback to a regular open and try to write to the file
	// 	f, err = fsys.Open(filename)
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	f, err := Create(fsys, filename)
	if errors.Is(err, ErrNotSupported) {
		f, err = fsys.Open(filename)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	n, err := Write(f, data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	if err1 := f.Close(); err == nil {
		err = err1
	}
	// TODO: use perm?
	return err
}
