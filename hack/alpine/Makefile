DOCKER_CMD 	?= $(shell command -v podman || command -v docker)

alpine.tgz: bundle
	tar -C bundle -czf alpine.tgz rootfs init.js

# go run fixlinks.go bundle/rootfs
bundle:
	mkdir -p bundle/rootfs
	$(DOCKER_CMD) build -t alpine-build --platform linux/386 -f Dockerfile .
	$(DOCKER_CMD) create --name alpine-export --platform linux/386 alpine-build
	$(DOCKER_CMD) export alpine-export | tar -C bundle/rootfs -xf -
	$(DOCKER_CMD) rm alpine-export
	cp init.js bundle/init.js

clean:
	rm -f alpine.tgz
	rm -rf bundle
.PHONY: clean
