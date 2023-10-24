.PHONY: boot dev kernel shell

all: kernel shell

kernel: kernel/main.go bin
	cd kernel && GOOS=js GOARCH=wasm go build -o ../bin/kernel .

shell: shell/main.go bin
	cd shell && GOOS=js GOARCH=wasm go build -o ../bin/shell .

bin:
	mkdir -p bin