.PHONY: boot dev kernel shell dev bundle micro hugo wanix boot/kernel.gz

VERSION=0.2dev
DEBUG?=false

all: wanix kernel shell build micro

dev: all
	./local/bin/wanix dev

wanix: local/bin/wanix
local/bin/wanix: kernel initfs
	go build -o ./local/bin/wanix ./cmd/wanix

kernel: boot/kernel.gz
boot/kernel.gz: 
	cd kernel && GOOS=js GOARCH=wasm go build -ldflags="-X 'main.Version=${VERSION}' -X 'tractor.dev/wanix/kernel/fs.DebugLog=${DEBUG}'" -o ../boot/kernel .
	gzip -f -9 ./boot/kernel

initfs: boot/initfs.gz
boot/initfs.gz: boot/initfs
	tar -czf ./boot/initfs.tar ./boot/initfs
	gzip -f -9 ./boot/initfs.tar
	mv ./boot/initfs.tar.gz ./boot/initfs.gz

boot/initfs: shell micro build
	mkdir -p boot/initfs
	cp -r shell boot/initfs
	cp -r internal/export boot/initfs

shell: boot/initfs/bin/shell
boot/initfs/bin/shell: shell/main.go
	cd shell && GOOS=js GOARCH=wasm go build -o ../boot/initfs/bin/shell .

micro: boot/initfs/bin/micro
boot/initfs/bin/micro: external/micro/
	make -C external/micro build

build/pkg.zip: build/build-pkgs/imports/imports.go build/build-pkgs/main.go
	cd build && go run ./build-pkgs/main.go ./build-pkgs/imports ./pkg.zip

build: boot/initfs/bin/build
boot/initfs/bin/build: build/main.go build/pkg.zip
	cd build && GOOS=js GOARCH=wasm go build -o ../boot/initfs/bin/build .

hugo: external/hugo/
	make -C external/hugo build
