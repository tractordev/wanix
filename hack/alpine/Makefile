DOCKER_CMD 	?= $(shell command -v podman || command -v docker)

alpine.tgz: bundle
	tar -C bundle -czf alpine.tgz rootfs init.js

bundle:
	mkdir -p bundle/rootfs
	$(DOCKER_CMD) create --name alpine-export --platform linux/386 i386/alpine:latest
	$(DOCKER_CMD) export alpine-export | tar -C bundle/rootfs -xf -
	$(DOCKER_CMD) rm alpine-export
	go run fixlinks.go bundle/rootfs
	cp bin/init bundle/rootfs/bin/init
	cp bin/post-dhcp bundle/rootfs/bin/post-dhcp
	cp init.js bundle/init.js

clean:
	rm -f alpine.tgz
	rm -rf bundle
.PHONY: clean
