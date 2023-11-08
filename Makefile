.PHONY: boot dev kernel shell dev bundle

all: kernel shell

dev:
	go run ./dev

bundle: local/bin
	go run -tags bundle ./dev

kernel: kernel/main.go local/bin
	cd kernel && GOOS=js GOARCH=wasm go build -o ../local/bin/kernel .

shell: shell/main.go local/bin
	cd shell && GOOS=js GOARCH=wasm go build -o ../local/bin/shell .

local/bin:
	mkdir -p local/bin