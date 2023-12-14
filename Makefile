.PHONY: boot dev kernel shell dev bundle micro

VERSION=0.1

all: kernel shell

dev: all
	go run ./dev

bundle: local/bin
	go run -tags bundle ./dev

kernel: kernel/main.go local/bin
	cd kernel && GOOS=js GOARCH=wasm go build -ldflags="-X 'main.Version=${VERSION}'" -o ../local/bin/kernel .

shell: shell/main.go local/bin
	cd shell && GOOS=js GOARCH=wasm go build -o ../local/bin/shell .

micro: external/micro/
	cd external/micro && make build
	mv external/micro/repo/micro.wasm local/bin/micro

local/bin:
	mkdir -p local/bin
