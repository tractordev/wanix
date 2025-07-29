.PHONY: build clobber

build:
	docker build $(if $(wildcard libv86.js),,--no-cache) --load -t wanix-v86-build .
	docker run --rm -v "$(shell pwd):/dst" wanix-v86-build

clobber:
	rm -f libv86.js
	rm -f v86.wasm
	rm -f seabios.bin
	rm -f vgabios.bin