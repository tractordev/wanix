DOCKER_CMD 	?= $(shell command -v docker)

dist: dist/rc.wasm
	$(DOCKER_CMD) build -t wanix-extras-build -f Dockerfile .
	$(DOCKER_CMD) rm -f wanix-extras-export || true && mkdir -p dist
	$(DOCKER_CMD) create --name wanix-extras-export wanix-extras-build
	$(DOCKER_CMD) cp wanix-extras-export:/dist .
.PHONY: dist

dist/rc.wasm:
	cd ../rc && make build
	cp ../rc/rc.wasm dist/rc.wasm

clean:
	rm -rf dist
.PHONY: clean
