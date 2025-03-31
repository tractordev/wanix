.PHONY: build clobber

build:
	GOOS=wasip1 GOARCH=wasm go build -o golangcheck.wasm .

clobber:
	rm -f golangcheck.wasm
