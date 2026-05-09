//go:build js && wasm

package main

import (
	"embed"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"syscall/js"

	_ "embed"

	"tractor.dev/wanix/misc/jsutil"
)

//go:embed lib.js
var assets embed.FS

func main() {
	flag.Parse()

	js.Global().Get("self").Call("addEventListener", "message", js.FuncOf(func(this js.Value, args []js.Value) any {
		// todo: handle screen/input/term changes
		// jsutil.Log("worker message:", args[0])
		return nil
	}))

	// NOTE: weird call Get on string error when file doesnt exist
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

	// 9P handler for the v86 emulator. v86's handle9p adapter dispatches
	// requests concurrently and correlates replies by 9P tag (see Wc in
	// lib.js), so we keep a tag-keyed map of per-request reply callbacks.
	// 9P header layout: size[4] type[1] tag[2] -> tag is at offset 5.
	readTag := func(buf js.Value) uint16 {
		return uint16(buf.Index(5).Int()) | uint16(buf.Index(6).Int())<<8
	}
	var (
		p9Mu        sync.Mutex
		p9Callbacks = make(map[uint16]js.Value)
	)
	js.Global().Get("worker").Get("p9").Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
		data := args[0].Get("data")
		tag := readTag(data)
		p9Mu.Lock()
		cb, ok := p9Callbacks[tag]
		if ok {
			delete(p9Callbacks, tag)
		}
		p9Mu.Unlock()
		if ok {
			cb.Invoke(data)
		}
		// Unknown tag: response after Tflush or duplicate -- silently drop.
		return nil
	}))
	p9handler := js.FuncOf(func(this js.Value, args []js.Value) any {
		req := args[0]
		cb := args[1]
		tag := readTag(req)
		p9Mu.Lock()
		if _, exists := p9Callbacks[tag]; exists {
			jsutil.Log("p9 tag collision on tag", tag)
		}
		p9Callbacks[tag] = cb
		p9Mu.Unlock()
		js.Global().Get("worker").Get("p9").Call("postMessage", req)
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
	if flag.Arg(0) != "" {
		cmdline = append(cmdline, "export="+flag.Arg(0))
	}
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

	exportch := js.Global().Get("MessageChannel").New()
	// Buffer 9p messages on virtio-console1 output and post complete messages to exportch.port1
	var (
		p9Buf     []byte
		p9MsgLen  int
		p9NeedLen = 4 // First 4 bytes of message is size (LE uint32)
		signaled  bool
	)
	// Run synchronously on the JS callback frame: there are no blocking
	// operations in the body, and spawning a goroutine per byte introduces
	// a race on the shared p9Buf/p9MsgLen accumulator (Go can interleave
	// goroutines at any syscall/js call, and v86 fires this listener once
	// per byte during a burst, so concurrent appends/resets corrupt frames).
	vm.Call("add_listener", "serial0-output-byte", js.FuncOf(func(this js.Value, args []js.Value) any {
		if !signaled {
			signaled = true
			exportch.Get("port1").Call("postMessage", args[0])
			return nil
		}
		b := args[0].Int()
		p9Buf = append(p9Buf, byte(b))

		// First 4 bytes encode the 9P message size (little-endian uint32).
		if len(p9Buf) == p9NeedLen {
			p9MsgLen = int(binary.LittleEndian.Uint32(p9Buf[:4]))
		}

		// Once a full message is accumulated, ship it over and reset.
		if p9MsgLen > 0 && len(p9Buf) == p9MsgLen {
			uint8arr := js.Global().Get("Uint8Array").New(p9MsgLen)
			js.CopyBytesToJS(uint8arr, p9Buf)
			exportch.Get("port1").Call("postMessage", uint8arr)
			p9Buf = p9Buf[:0]
			p9MsgLen = 0
		}
		return nil
	}))
	exportch.Get("port1").Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
		// jsutil.Log("in<<", args[0].Get("data"))
		// vm.Get("bus").Call("send", "virtio-console1-input-bytes", args[0].Get("data"))
		// vm.Get("bus").Call("send", "serial1-input", args[0].Get("data"))
		vm.Call("serial_send_bytes", "0", args[0].Get("data"))

		// test := js.Global().Get("Uint8Array").New(8)
		// js.CopyBytesToJS(test, []byte{5, 5, 4, 3, 3, 2, 1, 1})
		// jsutil.Log("in<<", test)
		// vm.Call("serial_send_bytes", "0", test)
		// vm.Call("serial_send_bytes", "1", test)
		return nil
	}))

	vm.Call("add_listener", "virtio-console0-output-bytes", js.FuncOf(func(this js.Value, args []js.Value) any {
		// ugh, these events are still one byte at a time?!?
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

		js.Global().Get("self").Call("postMessage", map[string]any{
			"vm":    os.Getenv("vm"),
			"guest": exportch.Get("port2"),
		}, []any{exportch.Get("port2")})

		return nil
	}))

	select {}
}
