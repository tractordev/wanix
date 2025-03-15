.PHONY: build clobber

build:
	docker build $(if $(wildcard shell.tgz),,--no-cache) --load -t wanix-shell-builder .
	docker run --rm -v "$(PWD):/output" wanix-shell-builder

clobber:
	rm -f shell.tgz
