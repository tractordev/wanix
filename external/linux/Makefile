.PHONY: build

build:
	docker build --load -t wanix-linux-builder .
	docker run --rm -v "$(PWD):/output" wanix-linux-builder
