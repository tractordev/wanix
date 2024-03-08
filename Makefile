.PHONY: boot dev kernel shell dev bundle micro hugo local/bin/kernel

VERSION=0.2dev
DEBUG?=false

all: kernel shell build micro

dev: all
	go run ./dev

bundle: local/bin/kernel
	cp local/bin/kernel local/wanix-kernel
	gzip -f -9 local/wanix-kernel
	go run -tags bundle ./dev

kernel: local/bin/kernel
local/bin/kernel: kernel/bin/shell kernel/bin/micro kernel/bin/build
	cd kernel && GOOS=js GOARCH=wasm go build -ldflags="-X 'main.Version=${VERSION}' -X 'tractor.dev/wanix/kernel/fs.DebugLog=${DEBUG}'" -o ../local/bin/kernel .

shell: kernel/bin/shell
kernel/bin/shell: shell/main.go
	cd shell && GOOS=js GOARCH=wasm go build -o ../kernel/bin/shell .

micro: kernel/bin/micro
kernel/bin/micro: external/micro/
	make -C external/micro build

hugo: external/hugo/
	make -C external/hugo build

build/pkg.zip: build/build-pkgs/imports/imports.go build/build-pkgs/main.go
	cd build && go run ./build-pkgs/main.go ./build-pkgs/imports ./pkg.zip

build: kernel/bin/build
kernel/bin/build: build/main.go build/pkg.zip
	cd build && GOOS=js GOARCH=wasm go build -o ../kernel/bin/build .
