DOCKER_CMD 	?= $(shell command -v podman || command -v docker)

bundle.tgz: bundle
	tar -C bundle -czf bundle.tgz .

bundle:
	$(DOCKER_CMD) build -t alpine-build --platform linux/386 -f Dockerfile .
	$(DOCKER_CMD) rm -f alpine-export && mkdir -p bundle
	$(DOCKER_CMD) create --name alpine-export --platform linux/386 alpine-build
	$(DOCKER_CMD) export alpine-export | tar -C bundle -xf -

clean:
	rm -f bundle.tgz
	rm -rf bundle
.PHONY: clean
