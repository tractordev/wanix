//go:build js && wasm

package vm

import (
	"fmt"
	"syscall/js"
)

func makeVM(id string) js.Value {
	options := map[string]any{
		"wasm_path":        "./v86/v86.wasm",
		"screen_container": js.Global().Get("document").Call("getElementById", "screen"),
		"memory_size":      1 * 1024 * 1024 * 1024, // 1GB
		"vga_memory_size":  8 * 1024 * 1024,        // 8MB
		"net_device": map[string]any{
			"type":      "ne2k",
			"relay_url": "ws://localhost:8777",
		},
		"bios": map[string]any{
			"url": "./v86/seabios.bin",
		},
		"vga_bios": map[string]any{
			"url": "./v86/vgabios.bin",
		},
		"bzimage": map[string]any{
			"url": "./linux/bzImage",
		},
		"cmdline": fmt.Sprintf("init=/bin/init rw root=host9p rootfstype=9p rootflags=trans=virtio,version=9p2000.L,aname=web/vm/%s/fsys,cache=none,msize=8192,access=client tsc=reliable mitigations=off random.trust_cpu=on ramdisk_size=102400", id),
	}
	vm := js.Global().Get("V86").New(options)
	readyPromise := js.Global().Get("Promise").New(js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		vm.Call("add_listener", "emulator-loaded", args[0])
		return nil
	}))
	vm.Set("ready", readyPromise)
	return vm
}
