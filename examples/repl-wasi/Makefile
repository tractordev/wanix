
build:
	@if ! command -v zig >/dev/null 2>&1; then \
		echo "skipping example build: zig not installed"; \
		exit 0; \
	fi; \
	zig build-exe repl.zig -target wasm32-wasi -O Debug
.PHONY: build
