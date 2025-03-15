.PHONY: build

build:
	docker build --load -t wanix-wasi-builder .
	docker run --rm -v "$(PWD):/output" wanix-wasi-builder
