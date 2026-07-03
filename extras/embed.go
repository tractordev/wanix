package extras

import (
	"embed"
)

//go:embed dist/rc.wasm dist/v86.tgz dist/wanix-linux.tgz
var Dir embed.FS
