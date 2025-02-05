package v86

import (
	"embed"
)

//go:embed libv86.js seabios.bin vgabios.bin v86.wasm
var Dir embed.FS
