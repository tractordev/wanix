//go:build js && wasm

package vm

import "syscall/js"

func makeVM() js.Value {
	options := map[string]any{
		"wasm_path":        "./v86/v86.wasm",
		"screen_container": js.Global().Get("document").Call("getElementById", "screen"),
		"memory_size":      512 * 1024 * 1024,
		"vga_memory_size":  8 * 1024 * 1024,
		"bios": map[string]any{
			"url": "./v86/seabios.bin",
		},
		"vga_bios": map[string]any{
			"url": "./v86/vgabios.bin",
		},
		"bzimage": map[string]any{
			"url": "./linux/bzImage",
		},
		"cmdline": "init=/bin/init rw root=host9p rootfstype=9p rootflags=trans=virtio,version=9p2000.L,aname=web/vm/1/fsys,cache=none,msize=8192,access=client tsc=reliable mitigations=off random.trust_cpu=on",
	}
	vm := js.Global().Get("V86").New(options)
	readyPromise := js.Global().Get("Promise").New(js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		vm.Call("add_listener", "emulator-loaded", args[0])
		return nil
	}))
	vm.Set("ready", readyPromise)
	return vm
}
