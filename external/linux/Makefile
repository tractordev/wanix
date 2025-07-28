.PHONY: build clobber

build:
	docker build $(if $(wildcard bzImage),,--no-cache) --load -t wanix-linux-builder .
	docker run --rm -v "$(shell pwd):/output" wanix-linux-builder

clobber:
	rm -f bzImage
