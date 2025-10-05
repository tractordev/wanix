// do not use this package, it is deprecated

package fsutil

import (
	"fmt"
	"os"
	"strings"
)

// FileMode represents all information contained in a file mode
type FileMode struct {
	// File Type
	IsRegular     bool
	IsDir         bool
	IsSymlink     bool
	IsNamedPipe   bool
	IsSocket      bool
	IsDevice      bool
	IsCharDevice  bool
	IsBlockDevice bool

	// Special Bits
	SetUID bool
	SetGID bool
	Sticky bool

	// User Permissions
	UserRead    bool
	UserWrite   bool
	UserExecute bool

	// Group Permissions
	GroupRead    bool
	GroupWrite   bool
	GroupExecute bool

	// Other Permissions
	OtherRead    bool
	OtherWrite   bool
	OtherExecute bool

	// Raw value
	Mode os.FileMode
}

// ParseFileMode parses an os.FileMode into a FileMode struct
func ParseFileMode(mode os.FileMode) *FileMode {
	fm := &FileMode{
		Mode: mode,
	}

	// File Type
	fm.IsRegular = mode.IsRegular()
	fm.IsDir = mode.IsDir()
	fm.IsSymlink = mode&os.ModeSymlink != 0
	fm.IsNamedPipe = mode&os.ModeNamedPipe != 0
	fm.IsSocket = mode&os.ModeSocket != 0
	fm.IsDevice = mode&os.ModeDevice != 0
	fm.IsCharDevice = mode&os.ModeCharDevice != 0
	fm.IsBlockDevice = mode&os.ModeDevice != 0 && mode&os.ModeCharDevice == 0

	// Special Bits
	fm.SetUID = mode&os.ModeSetuid != 0
	fm.SetGID = mode&os.ModeSetgid != 0
	fm.Sticky = mode&os.ModeSticky != 0

	// Permissions
	perm := mode.Perm()
	fm.UserRead = perm&0400 != 0
	fm.UserWrite = perm&0200 != 0
	fm.UserExecute = perm&0100 != 0
	fm.GroupRead = perm&0040 != 0
	fm.GroupWrite = perm&0020 != 0
	fm.GroupExecute = perm&0010 != 0
	fm.OtherRead = perm&0004 != 0
	fm.OtherWrite = perm&0002 != 0
	fm.OtherExecute = perm&0001 != 0

	return fm
}

// String returns a string representation of the FileMode
func (fm *FileMode) String() string {
	var parts []string

	// File Type
	typeStr := "unknown"
	switch {
	case fm.IsRegular:
		typeStr = "regular"
	case fm.IsDir:
		typeStr = "directory"
	case fm.IsSymlink:
		typeStr = "symlink"
	case fm.IsNamedPipe:
		typeStr = "named_pipe"
	case fm.IsSocket:
		typeStr = "socket"
	case fm.IsCharDevice:
		typeStr = "char_device"
	case fm.IsBlockDevice:
		typeStr = "block_device"
	}
	parts = append(parts, fmt.Sprintf("type=%s", typeStr))

	// Special Bits
	var special []string
	if fm.SetUID {
		special = append(special, "setuid")
	}
	if fm.SetGID {
		special = append(special, "setgid")
	}
	if fm.Sticky {
		special = append(special, "sticky")
	}
	if len(special) > 0 {
		parts = append(parts, fmt.Sprintf("special=[%s]", strings.Join(special, ",")))
	}

	// Permissions in octal
	parts = append(parts, fmt.Sprintf("perms=%04o", fm.Mode.Perm()))

	// Permissions in rwx format
	perms := ""
	if fm.UserRead {
		perms += "r"
	} else {
		perms += "-"
	}
	if fm.UserWrite {
		perms += "w"
	} else {
		perms += "-"
	}
	if fm.UserExecute {
		if fm.SetUID {
			perms += "s"
		} else {
			perms += "x"
		}
	} else {
		if fm.SetUID {
			perms += "S"
		} else {
			perms += "-"
		}
	}

	if fm.GroupRead {
		perms += "r"
	} else {
		perms += "-"
	}
	if fm.GroupWrite {
		perms += "w"
	} else {
		perms += "-"
	}
	if fm.GroupExecute {
		if fm.SetGID {
			perms += "s"
		} else {
			perms += "x"
		}
	} else {
		if fm.SetGID {
			perms += "S"
		} else {
			perms += "-"
		}
	}

	if fm.OtherRead {
		perms += "r"
	} else {
		perms += "-"
	}
	if fm.OtherWrite {
		perms += "w"
	} else {
		perms += "-"
	}
	if fm.OtherExecute {
		if fm.Sticky {
			perms += "t"
		} else {
			perms += "x"
		}
	} else {
		if fm.Sticky {
			perms += "T"
		} else {
			perms += "-"
		}
	}

	parts = append(parts, fmt.Sprintf("rwx=%s", perms))

	return fmt.Sprintf("FileMode{%s}", strings.Join(parts, " "))
}

// OpenFlags represents all information contained in open() flags
type OpenFlags struct {
	// Access Mode (mutually exclusive)
	ReadOnly  bool
	WriteOnly bool
	ReadWrite bool

	// File Creation Flags
	Create    bool
	Exclusive bool
	NoCtty    bool
	Truncate  bool

	// File Status Flags
	Append      bool
	Async       bool
	Sync        bool
	DataSync    bool
	NonBlock    bool
	Direct      bool
	Directory   bool
	NoFollow    bool
	NoAtime     bool
	CloseOnExec bool
	Path        bool
	TempFile    bool

	// Linux-specific flags (might not be available on all systems)
	LargeFile bool
	NoDelay   bool
	DSync     bool

	// Raw value
	Flags int
}

// ParseOpenFlags parses open() flags into an OpenFlags struct
func ParseOpenFlags(flags int) *OpenFlags {
	of := &OpenFlags{
		Flags: flags,
	}

	// Access Mode (bottom 2 bits)
	accMode := flags & 0x3
	of.ReadOnly = accMode == os.O_RDONLY
	of.WriteOnly = accMode == os.O_WRONLY
	of.ReadWrite = accMode == os.O_RDWR

	// File Creation Flags
	of.Create = flags&os.O_CREATE != 0
	of.Exclusive = flags&os.O_EXCL != 0
	// of.NoCtty = flags&unix.O_NOCTTY != 0
	of.Truncate = flags&os.O_TRUNC != 0

	// File Status Flags
	of.Append = flags&os.O_APPEND != 0
	// of.Async = flags&unix.O_ASYNC != 0
	of.Sync = flags&os.O_SYNC != 0
	// of.NonBlock = flags&unix.O_NONBLOCK != 0

	// Platform-specific flags (these constants might not exist on all platforms)
	// Using direct syscall numbers where available
	// if flags&unix.O_CLOEXEC != 0 {
	// of.CloseOnExec = true
	// }

	// Check for flags that might be platform-specific
	checkFlag := func(flag int, name string) bool {
		return flags&flag != 0
	}

	// Linux-specific flags (values may vary by platform)
	// O_DIRECT
	if checkFlag(0x4000, "O_DIRECT") || checkFlag(0x10000, "O_DIRECT") {
		of.Direct = true
	}

	// O_DIRECTORY
	if checkFlag(0x10000, "O_DIRECTORY") || checkFlag(0x20000, "O_DIRECTORY") {
		of.Directory = true
	}

	// O_NOFOLLOW
	if checkFlag(0x20000, "O_NOFOLLOW") || checkFlag(0x100, "O_NOFOLLOW") {
		of.NoFollow = true
	}

	// O_NOATIME (Linux)
	if checkFlag(0x40000, "O_NOATIME") {
		of.NoAtime = true
	}

	// O_PATH (Linux 2.6.39+)
	if checkFlag(0x200000, "O_PATH") {
		of.Path = true
	}

	// O_TMPFILE (Linux 3.11+)
	if checkFlag(0x400000, "O_TMPFILE") || checkFlag(0x410000, "O_TMPFILE") {
		of.TempFile = true
	}

	// O_DSYNC
	if checkFlag(0x1000, "O_DSYNC") {
		of.DSync = true
	}

	return of
}

// String returns a string representation of the OpenFlags
func (of *OpenFlags) String() string {
	var parts []string

	// Access Mode
	accessMode := "none"
	if of.ReadOnly {
		accessMode = "O_RDONLY"
	} else if of.WriteOnly {
		accessMode = "O_WRONLY"
	} else if of.ReadWrite {
		accessMode = "O_RDWR"
	}
	parts = append(parts, fmt.Sprintf("access=%s", accessMode))

	// Collect all set flags
	var flags []string
	if of.Create {
		flags = append(flags, "O_CREAT")
	}
	if of.Exclusive {
		flags = append(flags, "O_EXCL")
	}
	if of.NoCtty {
		flags = append(flags, "O_NOCTTY")
	}
	if of.Truncate {
		flags = append(flags, "O_TRUNC")
	}
	if of.Append {
		flags = append(flags, "O_APPEND")
	}
	if of.Async {
		flags = append(flags, "O_ASYNC")
	}
	if of.Sync {
		flags = append(flags, "O_SYNC")
	}
	if of.DataSync {
		flags = append(flags, "O_DSYNC")
	}
	if of.NonBlock {
		flags = append(flags, "O_NONBLOCK")
	}
	if of.Direct {
		flags = append(flags, "O_DIRECT")
	}
	if of.Directory {
		flags = append(flags, "O_DIRECTORY")
	}
	if of.NoFollow {
		flags = append(flags, "O_NOFOLLOW")
	}
	if of.NoAtime {
		flags = append(flags, "O_NOATIME")
	}
	if of.CloseOnExec {
		flags = append(flags, "O_CLOEXEC")
	}
	if of.Path {
		flags = append(flags, "O_PATH")
	}
	if of.TempFile {
		flags = append(flags, "O_TMPFILE")
	}
	if of.DSync {
		flags = append(flags, "O_DSYNC")
	}

	if len(flags) > 0 {
		parts = append(parts, fmt.Sprintf("flags=[%s]", strings.Join(flags, "|")))
	}

	// Add raw value in hex
	parts = append(parts, fmt.Sprintf("raw=0x%x", of.Flags))

	return fmt.Sprintf("OpenFlags{%s}", strings.Join(parts, " "))
}

// Detailed returns a detailed multi-line string representation
func (fm *FileMode) Detailed() string {
	var b strings.Builder

	b.WriteString("FileMode Details:\n")
	b.WriteString(fmt.Sprintf("  Raw Mode: %v (0%04o)\n", fm.Mode, fm.Mode))

	// File Type
	b.WriteString("  File Type:\n")
	if fm.IsRegular {
		b.WriteString("    - Regular File\n")
	}
	if fm.IsDir {
		b.WriteString("    - Directory\n")
	}
	if fm.IsSymlink {
		b.WriteString("    - Symbolic Link\n")
	}
	if fm.IsNamedPipe {
		b.WriteString("    - Named Pipe (FIFO)\n")
	}
	if fm.IsSocket {
		b.WriteString("    - Socket\n")
	}
	if fm.IsCharDevice {
		b.WriteString("    - Character Device\n")
	}
	if fm.IsBlockDevice {
		b.WriteString("    - Block Device\n")
	}

	// Special Bits
	if fm.SetUID || fm.SetGID || fm.Sticky {
		b.WriteString("  Special Bits:\n")
		if fm.SetUID {
			b.WriteString("    - SetUID: Set user ID on execution\n")
		}
		if fm.SetGID {
			b.WriteString("    - SetGID: Set group ID on execution\n")
		}
		if fm.Sticky {
			b.WriteString("    - Sticky: Restricted deletion (directories)\n")
		}
	}

	// Permissions
	b.WriteString("  Permissions:\n")
	b.WriteString(fmt.Sprintf("    User:  %s%s%s (owner)\n",
		boolToRWX(fm.UserRead, "r"),
		boolToRWX(fm.UserWrite, "w"),
		boolToRWX(fm.UserExecute, "x")))
	b.WriteString(fmt.Sprintf("    Group: %s%s%s\n",
		boolToRWX(fm.GroupRead, "r"),
		boolToRWX(fm.GroupWrite, "w"),
		boolToRWX(fm.GroupExecute, "x")))
	b.WriteString(fmt.Sprintf("    Other: %s%s%s (everyone else)\n",
		boolToRWX(fm.OtherRead, "r"),
		boolToRWX(fm.OtherWrite, "w"),
		boolToRWX(fm.OtherExecute, "x")))

	return b.String()
}

// Detailed returns a detailed multi-line string representation
func (of *OpenFlags) Detailed() string {
	var b strings.Builder

	b.WriteString("OpenFlags Details:\n")
	b.WriteString(fmt.Sprintf("  Raw Flags: 0x%x (%d)\n", of.Flags, of.Flags))

	// Access Mode
	b.WriteString("  Access Mode:\n")
	if of.ReadOnly {
		b.WriteString("    - O_RDONLY: Read-only access\n")
	}
	if of.WriteOnly {
		b.WriteString("    - O_WRONLY: Write-only access\n")
	}
	if of.ReadWrite {
		b.WriteString("    - O_RDWR: Read and write access\n")
	}

	// File Creation
	if of.Create || of.Exclusive || of.Truncate {
		b.WriteString("  File Creation:\n")
		if of.Create {
			b.WriteString("    - O_CREAT: Create file if it doesn't exist\n")
		}
		if of.Exclusive {
			b.WriteString("    - O_EXCL: Fail if file exists (with O_CREAT)\n")
		}
		if of.Truncate {
			b.WriteString("    - O_TRUNC: Truncate file to zero length\n")
		}
	}

	// File Status
	if of.Append || of.NonBlock || of.Sync || of.Async {
		b.WriteString("  File Status:\n")
		if of.Append {
			b.WriteString("    - O_APPEND: Append mode (writes go to end)\n")
		}
		if of.NonBlock {
			b.WriteString("    - O_NONBLOCK: Non-blocking I/O\n")
		}
		if of.Sync {
			b.WriteString("    - O_SYNC: Synchronous writes\n")
		}
		if of.Async {
			b.WriteString("    - O_ASYNC: Signal-driven I/O\n")
		}
	}

	// Advanced Flags
	if of.Direct || of.NoAtime || of.CloseOnExec || of.Directory || of.NoFollow || of.Path || of.TempFile {
		b.WriteString("  Advanced Flags:\n")
		if of.Direct {
			b.WriteString("    - O_DIRECT: Direct I/O (bypass cache)\n")
		}
		if of.NoAtime {
			b.WriteString("    - O_NOATIME: Don't update access time\n")
		}
		if of.CloseOnExec {
			b.WriteString("    - O_CLOEXEC: Close on exec() calls\n")
		}
		if of.Directory {
			b.WriteString("    - O_DIRECTORY: Must be a directory\n")
		}
		if of.NoFollow {
			b.WriteString("    - O_NOFOLLOW: Don't follow symlinks\n")
		}
		if of.Path {
			b.WriteString("    - O_PATH: Open for path operations only\n")
		}
		if of.TempFile {
			b.WriteString("    - O_TMPFILE: Create unnamed temporary file\n")
		}
	}

	return b.String()
}

func boolToRWX(b bool, char string) string {
	if b {
		return char
	}
	return "-"
}

// Example usage functions

// ParseFileStat parses file mode from os.Stat
func ParseFileStat(path string) (*FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return ParseFileMode(info.Mode()), nil
}
