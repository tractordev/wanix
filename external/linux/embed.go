package linux

import (
	"embed"
)

//go:embed bzImage
var Dir embed.FS
