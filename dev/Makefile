EXTERNALS := public/workbench public/assets/wanix.debug.wasm public/assets/wanix.min.js

build: $(EXTERNALS)
.PHONY: build

dev: $(EXTERNALS)
	wrangler dev
.PHONY: dev

clean:
	rm -rf ./public/workbench
	rm -rf ./public/assets/wanix.debug.wasm
	rm -rf ./public/assets/wanix.min.js
.PHONY: clean

public/workbench:
	ln -sf ../../workbench public/workbench

public/assets/wanix.debug.wasm:
	ln -sf ../../../dist/wanix.debug.wasm public/assets/wanix.debug.wasm

public/assets/wanix.min.js:
	ln -sf ../../../dist/wanix.min.js public/assets/wanix.min.js