NAME			?= wanix
VERSION 		?= v0.3-$(shell git rev-parse --short HEAD)
GOARGS			?=
GOOS			?= $(shell go env GOOS)
GOARCH			?= $(shell go env GOARCH)
WASM_GO 		?= tinygo
BIN 			?= /usr/local/bin
DIST_DIR		?= .local/dist
DIST_OS			?= darwin windows linux
DIST_ARCH		?= arm64 amd64

## Build dependencies and Wanix
all: deps build
.PHONY: all

## Build dependencies
deps: linux v86 wasi shell esbuild
.PHONY: deps

## Build Wanix (binary and module)
build: wasm wanix
.PHONY: build

## Build Wanix using Docker
docker: deps
	docker build --build-arg GOOS=$(GOOS) --build-arg GOARCH=$(GOARCH) --load -t wanix .
	docker run --rm -v "$(PWD):/output" wanix sh -c "cp ./wanix /output"
.PHONY: docker

## Build Wanix binary
wanix: wasm/assets/wanix.prebundle.js
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o wanix $(GOARGS) ./cmd/wanix
.PHONY: wanix

## Build WASM module
wasm: wasm-$(WASM_GO)
.PHONY: wasm

## Build WASM module using TinyGo
wasm-tinygo:
	tinygo build -target wasm -o wasm/assets/wanix.wasm ./wasm
	cp wasm/assets/wasm_exec.tinygo.js wasm/assets/wasm_exec.js
.PHONY: wasm-tinygo

## Build WASM module using Go
wasm-go:
	GOOS=js GOARCH=wasm go build -o wasm/assets/wanix.wasm ./wasm
	cp wasm/assets/wasm_exec.go.js wasm/assets/wasm_exec.js
.PHONY: wasm-go

## Build v86 emulator
v86:
	cd external/v86 && make build
.PHONY: v86

## Build Linux kernel
linux:
	cd external/linux && make build
.PHONY: linux

## Build esbuild
esbuild:
	cd external/esbuild && docker build --load -t esbuild .
.PHONY: esbuild

## Build shell for Wanix
shell:
	cd shell && make build
.PHONY: shell

## Build and bundle WASI module
wasi: 
	cd external/wasi && make build
	cp external/wasi/wasi.bundle.js wasm/assets/wasi/wasi.bundle.js
.PHONY: wasi

## Remove all built artifacts
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
.PHONY: clobber

wasm/assets/wanix.prebundle.js: wasm/assets/wanix.js
	docker run --rm -v $(PWD)/wasm/assets:/build esbuild wanix.js --bundle > wasm/assets/wanix.prebundle.js



.DEFAULT_GOAL := show-help

# Inspired by <http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html>
# sed script explained:
# /^##/:
# 	* save line in hold space
# 	* purge line
# 	* Loop:
# 		* append newline + line to hold space
# 		* go to next line
# 		* if line starts with doc comment, strip comment character off and loop
# 	* remove target prerequisites
# 	* append hold space (+ newline) to line
# 	* replace newline plus comments by `---`
# 	* print line
# Separate expressions are necessary because labels cannot be delimited by
# semicolon; see <http://stackoverflow.com/a/11799865/1968>
.PHONY: show-help
show-help:
	@echo "$$(tput bold)Available rules:$$(tput sgr0)"
	@echo
	@sed -n -e "/^## / { \
		h; \
		s/.*//; \
		:doc" \
		-e "H; \
		n; \
		s/^## //; \
		t doc" \
		-e "s/:.*//; \
		G; \
		s/\\n## /---/; \
		s/\\n/ /g; \
		p; \
	}" ${MAKEFILE_LIST} \
	| LC_ALL='C' sort --ignore-case \
	| awk -F '---' \
		-v ncol=$$(tput cols) \
		-v indent=19 \
		-v col_on="$$(tput setaf 6)" \
		-v col_off="$$(tput sgr0)" \
	'{ \
		printf "%s%*s%s ", col_on, -indent, $$1, col_off; \
		n = split($$2, words, " "); \
		line_length = ncol - indent; \
		for (i = 1; i <= n; i++) { \
			line_length -= length(words[i]) + 1; \
			if (line_length <= 0) { \
				line_length = ncol - indent - length(words[i]) - 1; \
				printf "\n%*s ", -indent, " "; \
			} \
			printf "%s ", words[i]; \
		} \
		printf "\n"; \
	}' \
	| more $(shell test $(shell uname) == Darwin && echo '--no-init --raw-control-chars')