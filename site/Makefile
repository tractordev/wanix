EXTERNALS := public/.lib/bench public/.lib/wanix.debug.wasm public/.lib/wanix.min.js

build: $(EXTERNALS)
.PHONY: build

dev: $(EXTERNALS)
	wrangler dev
.PHONY: dev

clean:
	rm -rf ./public/.lib/bench
	rm -rf ./public/.lib/wanix.debug.wasm
	rm -rf ./public/.lib/wanix.min.js
.PHONY: clean

public/.lib/bench:
	make -C ../workbench
	cp -r ../workbench public/.lib/bench

public/.lib/wanix.debug.wasm:
	cp ../dist/wanix.debug.wasm public/.lib/wanix.debug.wasm

public/.lib/wanix.min.js:
	cp ../dist/wanix.min.js public/.lib/wanix.min.js