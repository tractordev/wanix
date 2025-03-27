package assets

import (
	"embed"

	v86 "tractor.dev/wanix/external/v86"
)

//go:embed *
var Dir embed.FS

func WanixBundle() (out []byte) {
	libv86, err := v86.Dir.ReadFile("libv86.js")
	if err != nil {
		panic(err)
	}
	out = append(out, libv86...)

	wasmExec, err := Dir.ReadFile("wasm_exec.js")
	if err != nil {
		panic(err)
	}
	out = append(out, wasmExec...)

	wio, err := Dir.ReadFile("wio.js")
	if err != nil {
		panic(err)
	}
	out = append(out, wio...)

	prebundleJS, err := Dir.ReadFile("wanix.prebundle.js")
	if err != nil {
		panic(err)
	}
	out = append(out, prebundleJS...)

	return
}
