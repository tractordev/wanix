.PHONY: build clobber

build:
	docker build $(if $(wildcard bzImage),,--no-cache) --load -t wanix-linux-builder .
	docker run --rm -v "$(PWD):/output" wanix-linux-builder

clobber:
	rm -f bzImage
