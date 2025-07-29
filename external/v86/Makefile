DOCKER_CMD ?= $(shell command -v podman || command -v docker)

build:
	$(DOCKER_CMD) build $(if $(wildcard libv86.js),,--no-cache) --load -t wanix-build-v86 .
	$(DOCKER_CMD) run --rm -v "$(shell pwd):/dst" wanix-build-v86
.PHONY: build

clean:
	rm -f libv86.js
	rm -f v86.wasm
	rm -f seabios.bin
	rm -f vgabios.bin
.PHONY: clean