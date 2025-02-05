.PHONY: wanix wasm


wanix:
	go build -o wanix ./cmd/wanix

wasm:
	tinygo build -target wasm -o wasm/assets/wanix.wasm ./wasm
