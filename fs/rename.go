package fs

type RenameFS interface {
	FS
	Rename(oldname, newname string) error
}

// Rename renames (moves) oldname to newname if supported.
func Rename(fsys FS, oldname, newname string) error {
	if r, ok := fsys.(RenameFS); ok {
		return r.Rename(oldname, newname)
	}

	// TODO:
	// - find common parent and call Rename on it?
	// - check if resolved fsys for each name is the same?
	// - potentially use CopyFS and RemoveAll to move as last resort

	return opErr(fsys, newname, "rename", ErrNotSupported)
}
