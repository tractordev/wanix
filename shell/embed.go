package shell

import (
	"embed"
)

//go:embed shell.tgz
var Dir embed.FS
