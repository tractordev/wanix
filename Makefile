.PHONY: boot dev kernel shell dev bundle micro

VERSION=0.1
DEBUG?=false

all: kernel shell

dev: all
	go run ./dev

bundle: local/bin
	go run -tags bundle ./dev

kernel: kernel/main.go local/bin
	cd kernel && GOOS=js GOARCH=wasm go build -ldflags="-X 'main.Version=${VERSION}' -X 'tractor.dev/wanix/kernel/fs.DebugLog=${DEBUG}'" -o ../local/bin/kernel .

shell: shell/main.go local/bin
	cd shell && GOOS=js GOARCH=wasm go build -o ../local/bin/shell .

micro: external/micro/ local/bin
	cd external/micro && make build
	mv external/micro/repo/micro.wasm local/bin/micro

dev-micro: external/micro/ local/bin
	cd external/micro/repo/ && make wasm
	mv external/micro/repo/micro.wasm local/bin/micro

build/pkg.zip: build/build-pkgs/imports/imports.go build/build-pkgs/main.go
	cd build && go run ./build-pkgs/main.go ./build-pkgs/imports ./pkg.zip

build: build/main.go build/pkg.zip local/bin
	cd build && GOOS=js GOARCH=wasm go build -o ../local/bin/build .

local/bin:
	mkdir -p local/bin
