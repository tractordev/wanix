.PHONY: build clobber

build:
	GOOS=wasip1 GOARCH=wasm go build -o golang.wasm .

clobber:
	rm -f golang.wasm
