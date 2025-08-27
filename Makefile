# Makefile for Wanix
#
# This Makefile is used to build and manage the Wanix project. It is used to
# build the Linux kernel, v86 emulator, shell, and Wanix command. It is also
# used to build the distribution binaries.
#
# The Makefile is self-documenting, so you can run `make` to see all available
# tasks and their descriptions. The variables at the top can all be overridden
# by environment variables or by command line arguments (e.g. `make GOOS=linux`).
#
NAME			?= wanix
VERSION 		?= v0.3-$(shell git rev-parse --short HEAD)
GOARGS			?=
GOOS			?= $(shell go env GOOS)
GOARCH			?= $(shell go env GOARCH)
WASM_TOOLCHAIN 	?= $(shell basename $(shell command -v tinygo || command -v go))
LINK_BIN 		?= /usr/local/bin
DIST_DIR		?= .local/dist
DIST_OS			?= darwin windows linux
DIST_ARCH		?= arm64 amd64

export DOCKER_CMD 	?= $(shell command -v podman || command -v docker)
RUNTIME_TARGETS		:= runtime/assets/wanix.$(WASM_TOOLCHAIN).wasm runtime/assets/wanix.min.js runtime/wasi/worker/lib.js
DEP_TARGETS			:= shell/bundle.tgz

## Link/install the local Wanix command
link:
	[ -f "$(LINK_BIN)/$(NAME)" ] && rm "$(LINK_BIN)/$(NAME)" || true
	ln -fs "$(shell pwd)/.local/bin/$(NAME)" "$(LINK_BIN)/$(NAME)"
.PHONY: link

## Build dependencies and Wanix
all: deps build
.PHONY: all

## Build Linux kernel, v86 emulator, and shell
deps: deps-shell
.PHONY: deps

## Build Wanix (command and runtime)
build: runtime cmd
.PHONY: build

## Build Wanix (command and runtime) using Docker
build-docker: runtime-docker cmd-docker
.PHONY: build-docker


## Build Wanix command
cmd: $(DEP_TARGETS) $(RUNTIME_TARGETS)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o .local/bin/wanix $(GOARGS) ./cmd/wanix
.PHONY: cmd

## Build Wanix command using Docker
cmd-docker: $(DEP_TARGETS) $(RUNTIME_TARGETS)
	$(DOCKER_CMD) build --build-arg GOOS=$(GOOS) --build-arg GOARCH=$(GOARCH) --load -t wanix-build-cmd .
	$(DOCKER_CMD) run --rm -v "$(shell pwd)/.local/bin:/output" wanix-build-cmd sh -c "cp .local/bin/wanix /output"
.PHONY: docker


## Build WASM and JS modules
runtime: runtime-js runtime-wasm
.PHONY: runtime

## Build WASM and JS modules using Docker
runtime-docker:
	$(DOCKER_CMD) build --target runtime --load -t wanix-build-runtime -f Dockerfile.runtime .
	$(DOCKER_CMD) run --rm -v "$(shell pwd)/runtime:/output" wanix-build-runtime
.PHONY: runtime-docker

## Build WASM module
runtime-wasm: wasm-$(WASM_TOOLCHAIN)
.PHONY: wasm

## Build WASM module using TinyGo
wasm-tinygo: runtime/wasi/worker/lib.js
	tinygo build -target wasm -o runtime/assets/wanix.tinygo.wasm ./runtime/wasm
.PHONY: wasm-tinygo

## Build WASM module using Go
wasm-go: runtime/wasi/worker/lib.js
	GOOS=js GOARCH=wasm go build -o runtime/assets/wanix.go.wasm ./runtime/wasm
.PHONY: wasm-go

## Build JavaScript module (in Docker)
runtime-js:
	$(DOCKER_CMD) build --target js $(if $(wildcard runtime/assets/wanix.min.js),,--no-cache) --load -t wanix-build-js -f Dockerfile.runtime .
	$(DOCKER_CMD) run --rm -v "$(shell pwd)/runtime:/output" wanix-build-js
.PHONY: runtime-js

## Build shell for Wanix (in Docker)
deps-shell:
	make -C shell
.PHONY: deps-shell

## Remove dependency artifacts
deps-clean:
	make -C shell clean
.PHONY: deps-clean

## Remove Wanix runtime and command artifacts
clean:
	rm -f .local/bin/wanix
	rm -f runtime/assets/wanix.min.js
	rm -f runtime/assets/wanix.go.wasm
	rm -f runtime/assets/wanix.tinygo.wasm
	rm -f runtime/wasi/worker/lib.js
.PHONY: clean

## Remove all built artifacts
clobber:
	make deps-clean
	make clean
.PHONY: clobber

DIST_TARGETS	:= $(foreach os, $(DIST_OS), $(foreach arch, $(DIST_ARCH), $(DIST_DIR)/$(NAME)_$(VERSION)_$(os)_$(arch)))
$(DIST_TARGETS): $(DIST_DIR)/%:
	GOOS=$(word 3, $(subst _, ,$@)) \
	GOARCH=$(word 4, $(subst _, ,$@)) \
	go build -ldflags="-X main.Version=$(VERSION)" $(GOARGS) -o $@ ./cmd/wanix

## Build distribution binaries
dist: $(DIST_TARGETS)
.PHONY: dist

runtime/wasi/worker/lib.js:
	make runtime-js

runtime/assets/wanix.min.js:
	make runtime-js

runtime/assets/wanix.go.wasm:
	make wasm-go

runtime/assets/wanix.tinygo.wasm:
	make wasm-tinygo

shell/bundle.tgz:
	make deps-shell

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