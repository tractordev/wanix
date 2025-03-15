package fs

import (
	"errors"
	"fmt"
)

var (
	// new errors
	ErrNotSupported = errors.New("operation not supported")
	ErrNotEmpty     = errors.New("directory not empty")
)

func opErr(fsys FS, name string, op string, err error) error {
	return &PathError{Op: op, Path: name, Err: fmt.Errorf("%w from %T", err, fsys)}
}
