DOCKER_CMD 	?= $(shell command -v podman || command -v docker)

build:
	$(DOCKER_CMD) build $(if $(wildcard bzImage),,--no-cache) --load -t wanix-build-linux .
	$(DOCKER_CMD) run --rm -v "$(shell pwd):/output" wanix-build-linux
.PHONY: build

clean:
	rm -f bzImage
.PHONY: clean
