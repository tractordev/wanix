DOCKER_CMD	?= $(shell command -v podman || command -v docker)

build:
	$(DOCKER_CMD) build $(if $(wildcard shell.tgz),,--no-cache) --load -t wanix-shell-builder .
	$(DOCKER_CMD) run --rm -v "$(shell pwd):/output" wanix-shell-builder
.PHONY: build

clean:
	rm -f shell.tgz
.PHONY: clean
