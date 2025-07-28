.PHONY: build clobber wanixjs

build: wanixjs
	docker build $(if $(wildcard wasi.js),,--no-cache) --load -t wanix-wasi-builder .
	docker run --rm -v "$(shell pwd):/output" wanix-wasi-builder

wanixjs:
	mkdir -p wanixjs
	cp ../../wasm/assets/wanix.js wanixjs/
	cp ../../wasm/assets/duplex.min.js wanixjs/

clobber:
	rm -f wasi.bundle.js
	rm -rf wanixjs