
build:
	docker build $(if $(wildcard shell.tgz),,--no-cache) --load -t wanix-shell-builder .
	docker run --rm -v "$(shell pwd):/output" wanix-shell-builder
.PHONY: build

clean:
	rm -f shell.tgz
.PHONY: clean