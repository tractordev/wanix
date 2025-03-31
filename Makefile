.PHONY: all deps build clobber docker wanix wasm-tinygo wasm-go v86 linux wasi shell esbuild

GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
GOARGS?=

all: deps build

deps: linux v86 wasi shell esbuild

build: wasm-tinygo wanix

docker: deps
	docker build --build-arg GOOS=$(GOOS) --build-arg GOARCH=$(GOARCH) --load -t wanix .
	docker run --rm -v "$(PWD):/output" wanix sh -c "cp ./wanix /output"

wanix: wasm/assets/wanix.prebundle.js
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o wanix $(GOARGS) ./cmd/wanix

wasm-tinygo:
	tinygo build -target wasm -o wasm/assets/wanix.wasm ./wasm
	cp wasm/assets/wasm_exec.tinygo.js wasm/assets/wasm_exec.js

wasm-go:
	GOOS=js GOARCH=wasm go build -o wasm/assets/wanix.wasm ./wasm
	cp wasm/assets/wasm_exec.go.js wasm/assets/wasm_exec.js

v86:
	cd external/v86 && make build

linux:
	cd external/linux && make build

esbuild:
	cd external/esbuild && docker build --load -t esbuild .

shell:
	cd shell && make build

wasi: 
	cd external/wasi && make build
	cp external/wasi/wasi.bundle.js wasm/assets/wasi/wasi.bundle.js

clobber:
	rm -f wanix
	rm -f wasm/assets/wasi/wasi.js
	rm -f wasm/assets/wasm_exec.js
	rm -f wasm/assets/wanix.wasm
	rm -f wasm/assets/wanix.prebundle.js
	make -C external/linux clobber
	make -C external/v86 clobber
	make -C external/wasi clobber
	make -C shell clobber

wasm/assets/wanix.prebundle.js: wasm/assets/wanix.js
	docker run --rm -v $(PWD)/wasm/assets:/build esbuild wanix.js --bundle > wasm/assets/wanix.prebundle.js