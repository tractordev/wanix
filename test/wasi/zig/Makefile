.PHONY: build clobber

build:
	zig build-exe main.zig \
		-femit-bin=zigcheck.wasm \
		-target wasm32-wasi \
		-O ReleaseSmall \
		--stack 65536 \

clobber:
	rm -f zigcheck.wasm
	rm -f zigcheck.wasm.o
