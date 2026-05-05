DOCKER_CMD 	?= $(shell command -v docker)

dist: dist/rc.wasm dist/workbench dist/v86.tgz
	$(DOCKER_CMD) build -t wanix-extras-build -f Dockerfile .
	$(DOCKER_CMD) rm -f wanix-extras > /dev/null 2>&1 || true && mkdir -p dist
	$(DOCKER_CMD) create --name wanix-extras wanix-extras-build
	$(DOCKER_CMD) cp wanix-extras:/dist/alpine-linux.tgz ./dist/alpine-linux.tgz
	$(DOCKER_CMD) rm -f wanix-extras
.PHONY: dist

publish: dist
	npm publish --tag alpha
.PHONY: publish

dist/rc.wasm:
	mkdir -p dist
	cd ../rc && make build
	cp ../rc/rc.wasm dist/rc.wasm

dist/v86.tgz:
	mkdir -p dist
	cd ../v86 && make
	cp ../v86/v86.tgz dist/v86.tgz

dist/workbench:
	mkdir -p dist
	cd ../workbench && pwd && make build
	cp -r ../workbench dist/workbench
	rm -rf dist/workbench/node_modules
	rm dist/workbench/.gitignore

clean:
	rm -rf dist
.PHONY: clean
