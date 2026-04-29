DOCKER_CMD 	?= $(shell command -v docker)

dist: dist/rc.wasm dist/workbench
	$(DOCKER_CMD) build -t wanix-extras-build -f Dockerfile .
	$(DOCKER_CMD) rm -f wanix-extras-export || true && mkdir -p dist
	$(DOCKER_CMD) create --name wanix-extras-export wanix-extras-build
	$(DOCKER_CMD) cp wanix-extras-export:/dist .
.PHONY: dist

publish: dist
	npm publish --tag alpha
.PHONY: publish

dist/rc.wasm:
	mkdir -p dist
	cd ../rc && make build
	cp ../rc/rc.wasm dist/rc.wasm

dist/workbench:
	mkdir -p dist
	cd ../workbench && pwd && make build
	cp -r ../workbench dist/workbench
	rm -rf dist/workbench/node_modules
	rm dist/workbench/.gitignore

clean:
	rm -rf dist
.PHONY: clean
