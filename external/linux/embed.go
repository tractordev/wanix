package linux

import (
	"embed"
)

//go:embed bzImage initramfs.gz
var Dir embed.FS
