package fs

import (
	"errors"
	iofs "io/fs"
)

var (
	ErrInvalid    = iofs.ErrInvalid
	ErrPermission = iofs.ErrPermission
	ErrExist      = iofs.ErrExist
	ErrNotExist   = iofs.ErrNotExist
	ErrClosed     = iofs.ErrClosed

	// new errors
	ErrNotSupported = errors.New("operation not supported")
	ErrNotEmpty     = errors.New("directory not empty")

	SkipAll = iofs.SkipAll
	SkipDir = iofs.SkipDir
)

var (
	FormatDirEntry     = iofs.FormatDirEntry
	FormatFileInfo     = iofs.FormatFileInfo
	Glob               = iofs.Glob
	ReadFile           = iofs.ReadFile
	ValidPath          = iofs.ValidPath
	WalkDir            = iofs.WalkDir
	FileInfoToDirEntry = iofs.FileInfoToDirEntry
	ReadDir            = iofs.ReadDir
	Stat               = iofs.Stat
	// Sub                = iofs.Sub

)

const (
	ModeDir        = iofs.ModeDir
	ModeAppend     = iofs.ModeAppend
	ModeExclusive  = iofs.ModeExclusive
	ModeTemporary  = iofs.ModeTemporary
	ModeSymlink    = iofs.ModeSymlink
	ModeDevice     = iofs.ModeDevice
	ModeNamedPipe  = iofs.ModeNamedPipe
	ModeSocket     = iofs.ModeSocket
	ModeSetuid     = iofs.ModeSetuid
	ModeSetgid     = iofs.ModeSetgid
	ModeCharDevice = iofs.ModeCharDevice
	ModeSticky     = iofs.ModeSticky
	ModeIrregular  = iofs.ModeIrregular
	ModeType       = iofs.ModeType
	ModePerm       = iofs.ModePerm
)

type (
	DirEntry    = iofs.DirEntry
	FS          = iofs.FS
	File        = iofs.File
	FileInfo    = iofs.FileInfo
	FileMode    = iofs.FileMode
	GlobFS      = iofs.GlobFS
	PathError   = iofs.PathError
	ReadDirFS   = iofs.ReadDirFS
	ReadDirFile = iofs.ReadDirFile
	ReadFileFS  = iofs.ReadFileFS
	StatFS      = iofs.StatFS
	SubFS       = iofs.SubFS
	WalkDirFunc = iofs.WalkDirFunc
)
