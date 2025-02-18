package fs

type TruncateFS interface {
	FS
	Truncate(name string, size int64) error
}

func Truncate(fsys FS, name string, size int64) error {
	if t, ok := fsys.(TruncateFS); ok {
		return t.Truncate(name, size)
	}

	b, err := ReadFile(fsys, name)
	if err != nil {
		return err
	}

	return WriteFile(fsys, name, b[:size], 0)
}
