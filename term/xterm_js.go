//go:build js && wasm

package term

import "tractor.dev/wanix/vnd/xterm"

func loadXtermCSS() {
	xterm.Load()
}
