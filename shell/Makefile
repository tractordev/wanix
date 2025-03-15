.PHONY: build

build:
	docker build --load -t wanix-shell-builder .
	docker run --rm -v "$(PWD):/output" wanix-shell-builder
