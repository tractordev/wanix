//go:build js && wasm

package term

import "tractor.dev/wanix/misc/xterm"

func loadXtermCSS() {
	xterm.Load()
}
