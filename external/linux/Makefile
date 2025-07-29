
build:
	docker build $(if $(wildcard bzImage),,--no-cache) --load -t wanix-build-linux .
	docker run --rm -v "$(shell pwd):/output" wanix-build-linux
.PHONY: build

clean:
	rm -f bzImage
.PHONY: clean