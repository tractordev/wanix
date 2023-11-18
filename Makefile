.PHONY: boot dev kernel shell dev bundle

all: kernel shell build

dev:
	go run ./dev

bundle: local/bin
	go run -tags bundle ./dev

kernel: kernel/main.go local/bin
	cd kernel && GOOS=js GOARCH=wasm go build -o ../local/bin/kernel .

shell: shell/main.go local/bin
	cd shell && GOOS=js GOARCH=wasm go build -o ../local/bin/shell .

build: build/main.go build/pkg.zip local/bin
	cd build && GOOS=js GOARCH=wasm go build -o ../local/bin/build .
# 	cd build && GOOS=js GOARCH=wasm go build -o ./build .
# 	cd build && cat ./build ./pkg.zip > ../local/bin/build && rm ./build

local/bin:
	mkdir -p local/bin
