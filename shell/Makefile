.PHONY: build

build:
	docker build -t wanix-shell-builder .
	docker run --rm -v "$(PWD):/output" wanix-shell-builder
