package pstat

import "io/fs"

// UnixModeToFileMode converts Unix file mode to Go fs.FileMode
func UnixModeToFileMode(unixMode uint32) fs.FileMode {
	// Extract permission bits (lower 9 bits)
	perm := fs.FileMode(unixMode & 0o777)

	// Extract file type from Unix mode
	switch unixMode & 0o170000 { // S_IFMT mask
	case 0o40000: // S_IFDIR - directory
		return fs.ModeDir | perm
	case 0o120000: // S_IFLNK - symbolic link
		return fs.ModeSymlink | perm
	case 0o60000: // S_IFBLK - block device
		return fs.ModeDevice | perm
	case 0o20000: // S_IFCHR - character device
		return fs.ModeCharDevice | perm
	case 0o10000: // S_IFIFO - named pipe (FIFO)
		return fs.ModeNamedPipe | perm
	case 0o140000: // S_IFSOCK - socket
		return fs.ModeSocket | perm
	case 0o100000: // S_IFREG - regular file
		fallthrough
	default:
		return perm
	}
}

// FileModeToUnixMode converts Go fs.FileMode to Unix file mode
func FileModeToUnixMode(mode fs.FileMode) uint32 {
	// Start with permission bits
	unixMode := uint32(mode & fs.ModePerm) // 0o777

	// Add file type bits
	if mode&fs.ModeDir != 0 {
		unixMode |= 0o40000 // S_IFDIR
	} else if mode&fs.ModeSymlink != 0 {
		unixMode |= 0o120000 // S_IFLNK
	} else if mode&fs.ModeDevice != 0 {
		unixMode |= 0o60000 // S_IFBLK
	} else if mode&fs.ModeCharDevice != 0 {
		unixMode |= 0o20000 // S_IFCHR
	} else if mode&fs.ModeNamedPipe != 0 {
		unixMode |= 0o10000 // S_IFIFO
	} else if mode&fs.ModeSocket != 0 {
		unixMode |= 0o140000 // S_IFSOCK
	} else {
		unixMode |= 0o100000 // S_IFREG - regular file
	}

	return unixMode
}
