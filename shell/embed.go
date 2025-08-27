package shell

import (
	"embed"
)

//go:embed bundle.tgz
var Dir embed.FS
