
build: golangcheck.tgz
.PHONY: build

golangcheck.tgz: golangcheck.wasm init.js
	tar -czf golangcheck.tgz golangcheck.wasm init.js

golangcheck.wasm: main.go
	GOOS=wasip1 GOARCH=wasm go build -o golangcheck.wasm .

clean:
	rm -f golangcheck.wasm
	rm -f golangcheck.tgz
.PHONY: clean