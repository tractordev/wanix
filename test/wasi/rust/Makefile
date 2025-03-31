.PHONY: build clobber

build:
	rustc --target wasm32-wasip1 -o rustcheck.wasm main.rs

clobber:
	rm -f rustcheck.wasm