.PHONY: build clobber

build:
	zig build-exe main.zig -target wasm32-wasi -O ReleaseSmall -femit-bin=zig.wasm 

clobber:
	rm -f zig.wasm
	rm -f zig.wasm.o
