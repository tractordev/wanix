package fs

// BindMode controls the order of bindings at a destination path.
type BindMode int

const (
	ModeReplace BindMode = 0
	ModeAfter   BindMode = 1
	ModeBefore  BindMode = -1
)

// BindFS is implemented by filesystems that support Plan9-style bind operations.
type BindFS interface {
	FS
	Bind(src FS, srcPath, dstPath string, mode ...BindMode) error
}

// UnbindFS is implemented by filesystems that support removing bindings.
type UnbindFS interface {
	FS
	Unbind(src FS, srcPath, dstPath string) error
}

// Bind adds a file or directory binding if supported.
func Bind(fsys FS, src FS, srcPath, dstPath string, mode ...BindMode) error {
	if b, ok := fsys.(BindFS); ok {
		return b.Bind(src, srcPath, dstPath, mode...)
	}
	return opErr(fsys, dstPath, "bind", ErrNotSupported)
}

// Unbind removes a binding if supported.
func Unbind(fsys FS, src FS, srcPath, dstPath string) error {
	if u, ok := fsys.(UnbindFS); ok {
		return u.Unbind(src, srcPath, dstPath)
	}
	return opErr(fsys, dstPath, "unbind", ErrNotSupported)
}
