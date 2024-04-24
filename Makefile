.PHONY: boot dev kernel shell dev bundle micro hugo wanix boot/kernel.gz initfsDirs clean

VERSION=0.2dev
DEBUG?=false

all: wanix kernel shell build micro

dev: all
	./local/bin/wanix dev

clean:
	rm -rf ./boot/initfs
	rm -rf ./boot/initfs.gz
	rm -rf ./boot/kernel.gz

loader: all
	cd ./local && ./bin/wanix loader

wanix: local/bin/wanix
local/bin/wanix: kernel initfs
	go build -o ./local/bin/ ./cmd/wanix

kernel: boot/kernel.gz
boot/kernel.gz: 
	cd kernel && GOOS=js GOARCH=wasm go build -ldflags="-X 'main.Version=${VERSION}' -X 'tractor.dev/wanix/kernel/fs.DebugLog=${DEBUG}'" -o ../boot/kernel .
	gzip -f -9 ./boot/kernel

initfs: boot/initfs.gz
boot/initfs.gz: boot/initfs
	tar -cf ./boot/initfs.tar  -C ./boot/initfs .
	gzip -f -9 ./boot/initfs.tar
	mv ./boot/initfs.tar.gz ./boot/initfs.gz

boot/initfs: initfsDirs shell micro build
	cp -r ./shell ./boot/initfs/cmd/
	cp internal/export/exportapp.sh ./boot/initfs/cmd/
	cp internal/export/main.go ./boot/initfs/export/

initfsDirs:
	mkdir -p ./boot/initfs
	mkdir -p ./boot/initfs/bin
	mkdir -p ./boot/initfs/cmd
	mkdir -p ./boot/initfs/export

shell: boot/initfs/bin/shell.wasm
boot/initfs/bin/shell.wasm: shell/main.go
	cd shell && GOOS=js GOARCH=wasm go build -o ../boot/initfs/bin/shell.wasm .

micro: boot/initfs/cmd/micro.wasm
boot/initfs/cmd/micro.wasm: external/micro/
	make -C external/micro build

build/pkg.zip: build/build-pkgs/imports/imports.go build/build-pkgs/main.go
	cd build && go run ./build-pkgs/main.go ./build-pkgs/imports ./pkg.zip

build: boot/initfs/cmd/build.wasm
boot/initfs/cmd/build.wasm: build/main.go build/pkg.zip
	cd build && GOOS=js GOARCH=wasm go build -o ../boot/initfs/cmd/build.wasm .

hugo: external/hugo/
	make -C external/hugo build
