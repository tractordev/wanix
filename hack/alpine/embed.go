package alpine

import (
	"embed"
)

//go:embed alpine.tgz
var Dir embed.FS
