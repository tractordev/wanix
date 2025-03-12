.PHONY: all wanix wasm-tinygo wasm-go v86 linux wasi shell

all: linux v86 wasi wasm-tinygo wanix shell

build: wasm-tinygo wanix

wanix:
	go build -o wanix ./cmd/wanix

wasm-tinygo:
	tinygo build -target wasm -o wasm/assets/wanix.wasm ./wasm
	cp wasm/assets/wasm_exec.tinygo.js wasm/assets/wasm_exec.js

wasm-go:
	GOOS=js GOARCH=wasm go build -o wasm/assets/wanix.wasm ./wasm
	cp wasm/assets/wasm_exec.go.js wasm/assets/wasm_exec.js

v86:
	cd external/v86 && make build

linux:
	cd external/linux && make build

shell:
	cd shell && make build

wasi: 
	cd external/wasi && make build
	cp external/wasi/wasi.js wasm/assets/wasi/wasi.js
