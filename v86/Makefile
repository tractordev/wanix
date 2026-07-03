export DOCKER_CMD 	?= $(shell command -v docker)

build: wasm bundle
.PHONY: build

extra: v86.tgz
	@if [ -d ../extras/dist ]; then cp v86.tgz ../extras/dist/; fi
.PHONY: extra

wasm: bundle/v86-vm.wasm
	GOWORK=off GOOS=js GOARCH=wasm go build -o ./bundle/v86-vm.wasm .
.PHONY: wasm

bundle: bundle/v86-vm.wasm bundle/v86.wasm
	tar -C bundle -czf v86.tgz .
.PHONY: bundle

v86.tgz: bundle	

bundle/v86-vm.wasm: lib.js

bundle/v86.wasm:
	mkdir -p ./bundle
	$(DOCKER_CMD) pull --platform=linux/amd64 ghcr.io/progrium/v86:latest
	$(DOCKER_CMD) rm -f v86-bundle > /dev/null 2>&1 || true 
	$(DOCKER_CMD) create --name v86-bundle ghcr.io/progrium/v86:latest true
	$(DOCKER_CMD) cp v86-bundle:/v86.wasm ./bundle/v86.wasm
	$(DOCKER_CMD) cp v86-bundle:/bios/vgabios.bin ./bundle/vgabios.bin
	$(DOCKER_CMD) cp v86-bundle:/bios/seabios.bin ./bundle/seabios.bin
	$(DOCKER_CMD) rm -f v86-bundle

lib.js:
	go run lib/build.go

clean:
	rm -rf bundle
	rm -f lib.js
	rm -f v86.tgz
.PHONY: clean

