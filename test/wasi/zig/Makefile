.PHONY: build clobber

build:
	zig build-exe main.zig -target wasm32-wasi -lc -O ReleaseSmall -femit-bin=zigcheck.wasm 

clobber:
	rm -f zigcheck.wasm
	rm -f zigcheck.wasm.o
