.PHONY: build

build:
	docker build -t wanix-wasi-builder .
	docker run --rm -v "$(PWD):/output" wanix-wasi-builder
