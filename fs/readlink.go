package fs

import (
	"errors"
	"io"
	"path"
	"strings"
)

type ReadlinkFS interface {
	FS
	Readlink(name string) (string, error)
}

func Readlink(fsys FS, name string) (string, error) {
	if c, ok := fsys.(ReadlinkFS); ok {
		return c.Readlink(name)
	}

	ctx := WithReadOnly(ContextFor(fsys))
	rfsys, rname, err := ResolveTo[ReadlinkFS](fsys, ctx, name)
	if err == nil {
		return rfsys.Readlink(rname)
	}
	if !errors.Is(err, ErrNotSupported) {
		return "", opErr(fsys, name, "readlink", err)
	}

	f, err := OpenContext(WithNoFollow(ctx), fsys, name)
	if err != nil {
		return "", err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", err
	}
	if fi.Mode()&ModeSymlink == 0 {
		return "", ErrInvalid
	}

	b, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(b)), nil
}

// deprecated?
func targetNormalizer(symlink string) func(target string, err error) (string, error) {
	return func(target string, err error) (normalized string, e error) {
		if err != nil {
			e = err
			return
		}
		normalized = target
		if normalized == "/" {
			normalized = "."
			return
		}
		if strings.HasPrefix(normalized, "./") {
			normalized = path.Join(path.Dir(symlink), normalized[2:])
		}
		// if strings.HasPrefix(normalized, "/") {
		// 	normalized = normalized[1:]
		// }
		return
	}
}
