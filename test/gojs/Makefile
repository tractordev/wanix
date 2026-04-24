
build: gojscheck.wasm
.PHONY: build

gojscheck.wasm: main.go
	GOOS=js GOARCH=wasm go build -o gojscheck.wasm .

clean:
	rm -f gojscheck.wasm
.PHONY: clean