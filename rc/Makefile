
build:
	GOWORK=off GOOS=js GOARCH=wasm go build -o ./rc.wasm ./cmd/rc
.PHONY: build

clean:
	rm -f rc.wasm
.PHONY: clean