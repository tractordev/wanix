DOCKER_CMD		?= $(shell command -v podman || command -v docker)
DEP_TARGETS		:= vm/v86/bzImage vm/v86/v86.wasm

build: $(DEP_TARGETS)
	$(DOCKER_CMD) build $(if $(wildcard shell.tgz),,--no-cache) --load -t wanix-shell-builder .
	$(DOCKER_CMD) run --rm -v "$(shell pwd):/output" wanix-shell-builder
.PHONY: build

clean:
	rm -f shell.tgz
	rm -rf vm
.PHONY: clean

vm/v86/bzImage: ../external/linux/bzImage
	mkdir -p vm/v86
	cp ../external/linux/bzImage vm/v86/

../external/linux/bzImage:
	make -C ../external/linux build

vm/v86/v86.wasm: ../external/v86/v86.wasm
	mkdir -p vm/v86
	cp ../external/v86/libv86.js vm/v86/
	cp ../external/v86/v86.wasm vm/v86/
	cp ../external/v86/seabios.bin vm/v86/
	cp ../external/v86/vgabios.bin vm/v86/

../external/v86/v86.wasm:
	make -C ../external/v86 build