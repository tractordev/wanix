//go:build js

package imports

// ======================== IMPORTANT ========================
// Run `go mod tidy` in this directory after modifying imports.
// You will get compile errors if you don't!

import (
	"tractor.dev/wanix/internal/jsutil"
	"tractor.dev/wanix/kernel/proc/exec"
)
