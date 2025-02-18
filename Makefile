.PHONY: all wanix wasm-tinygo wasm-go

all: wasm-tinygo wanix 

wanix:
	go build -o wanix ./cmd/wanix

wasm-tinygo:
	tinygo build -target wasm -o wasm/assets/wanix.wasm ./wasm
	cp wasm/assets/wasm_exec.tinygo.js wasm/assets/wasm_exec.js

wasm-go:
	GOOS=js GOARCH=wasm go build -o wasm/assets/wanix.wasm ./wasm
	cp wasm/assets/wasm_exec.go.js wasm/assets/wasm_exec.js
