//go:build js && wasm

package main

import (
	"embed"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"syscall/js"

	_ "embed"

	"tractor.dev/wanix/misc/jsutil"
)

//go:embed lib.js
var assets embed.FS

func main() {
	js.Global().Get("self").Call("addEventListener", "message", js.FuncOf(func(this js.Value, args []js.Value) any {
		// todo: handle screen/input/term changes
		jsutil.Log("worker message:", args[0])
		return nil
	}))

	v86wasm, err := jsutil.AwaitErr(js.Global().Get("sys").Call("readFile", "#vm/v86/v86.wasm"))
	if err != nil {
		log.Fatal(err)
	}
	v86bios, err := jsutil.AwaitErr(js.Global().Get("sys").Call("readFile", "#vm/v86/seabios.bin"))
	if err != nil {
		log.Fatal(err)
	}
	v86vgabios, err := jsutil.AwaitErr(js.Global().Get("sys").Call("readFile", "#vm/v86/vgabios.bin"))
	if err != nil {
		log.Fatal(err)
	}
	// weird call Get on string error when file doesnt exist
	bzImage, err := jsutil.AwaitErr(js.Global().Get("sys").Call("readFile", "boot/bzImage"))
	if err != nil {
		log.Fatal(err)
	}

	// Dynamically import libv86.mjs by creating a Blob URL and using JS dynamic import.
	// Create a Blob from the embedded libv86 bytes
	jslib, err := assets.ReadFile("lib.js")
	if err != nil {
		log.Fatal(err)
	}
	jsbuf := js.Global().Get("Uint8Array").New(len(jslib))
	js.CopyBytesToJS(jsbuf, jslib)
	jslibBlob := js.Global().Get("Blob").New(
		[]any{"var thisWorker = self; var process = undefined;", jsbuf},
		map[string]any{"type": "application/javascript"},
	)
	jslibURL := js.Global().Get("URL").Call("createObjectURL", jslibBlob)

	jsmod, err := jsutil.AwaitErr(js.Global().Call("eval", "(url)=>import(url)").Invoke(jslibURL))
	if err != nil {
		log.Fatalf("failed to import lib.js: %v", err)
	}

	wasmBlob := js.Global().Get("Blob").New([]any{v86wasm}, map[string]any{"type": "application/wasm"})
	wasmURL := js.Global().Get("URL").Call("createObjectURL", wasmBlob)

	bufObj := func(buf js.Value) map[string]any {
		return map[string]any{
			"buffer": buf.Call("slice").Get("buffer"),
		}
	}

	var p9SendCallback js.Value = js.Undefined()
	js.Global().Get("worker").Get("p9").Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
		if !p9SendCallback.IsUndefined() {
			p9SendCallback.Invoke(args[0].Get("data"))
		}
		return nil
	}))
	p9handler := js.FuncOf(func(this js.Value, args []js.Value) any {
		p9SendCallback = args[1]
		js.Global().Get("worker").Get("p9").Call("postMessage", args[0])
		return nil
	})

	cmdline := []string{
		"console=hvc0",
		"init=/bin/init",
		"rw",
		"root=host9p",
		"rootfstype=9p",
		"rootflags=trans=virtio,version=9p2000.L,aname=,cache=none,msize=131072",
		"loglevel=3",
	} // mem=1008M memmap=16M$1008M
	opts := map[string]any{
		"memory_size":     1024 * 1024 * 1024, // 1GB,
		"vga_memory_size": 8 * 1024 * 1024,    // 8MB
		"cmdline":         strings.Join(cmdline, " "),
		"autostart":       true,
		"wasm_path":       wasmURL,
		"filesystem": map[string]any{
			"handle9p": p9handler,
		},
		"bios":                           bufObj(v86bios),
		"vga_bios":                       bufObj(v86vgabios),
		"bzimage":                        bufObj(bzImage),
		"bzimage_initrd_from_filesystem": false,
		"disable_speaker":                true,
		"disable_mouse":                  true,
		"disable_keyboard":               false,
		"virtio_console":                 true,
		"net_device": map[string]any{
			"type": "virtio",
		},
	}

	vm := jsmod.Get("V86").New(opts)

	vm.Call("add_listener", "virtio-console0-output-bytes", js.FuncOf(func(this js.Value, args []js.Value) any {
		go func() {
			jsBuf := args[0]
			buf := make([]byte, jsBuf.Get("byteLength").Int())
			js.CopyBytesToGo(buf, jsBuf)
			fmt.Fprint(os.Stdout, string(buf))
		}()
		return nil
	}))

	sendStdin := func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				jsBuf := js.Global().Get("Uint8Array").New(n)
				js.CopyBytesToJS(jsBuf, buf[:n])
				vm.Get("bus").Call("send", "virtio-console0-input-bytes", jsBuf)
			}
			if err != nil {
				if err != io.EOF {
					log.Println("stdin read error:", err)
				}
				break
			}
		}
	}

	vm.Call("add_listener", "emulator-ready", js.FuncOf(func(this js.Value, args []js.Value) any {

		vm.Get("bus").Call("send", "virtio-console0-resize", []any{
			100, 100,
		})

		tmpScreen := js.Global().Get("OffscreenCanvas").New(800, 600)
		screenAdapter := jsmod.Get("OffscreenScreenAdapter").New(tmpScreen, js.FuncOf(func(this js.Value, args []js.Value) any {
			vm.Get("v86").Get("cpu").Get("devices").Get("vga").Call("screen_fill_buffer")
			return nil
		}))
		vm.Get("v86").Get("cpu").Get("devices").Get("vga").Set("screen", screenAdapter)
		vm.Set("screen_adapter", screenAdapter)

		go sendStdin()

		jsutil.Log("v86 - vm ready")
		return nil
	}))

	select {}
}
