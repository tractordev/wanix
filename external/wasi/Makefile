.PHONY: build clobber

build:
	docker build $(if $(wildcard wasi.js),,--no-cache) --load -t wanix-wasi-builder .
	docker run --rm -v "$(PWD):/output" wanix-wasi-builder

clobber:
	rm -f wasi.js
