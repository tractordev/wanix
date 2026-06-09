# Makefile for Wanix
#
# This Makefile is used to build and manage the Wanix project. It is used to
# build the Wanix shell, WASM and JS modules, and Wanix command. It is also
# used to build the distribution binaries.
#
# The Makefile is self-documenting, so you can run `make` to see all available
# tasks and their descriptions. The variables at the top can all be overridden
# by environment variables or by command line arguments (e.g. `make GOOS=linux`).
#
NAME			?= wanix
VERSION 		?= v0.4-$(shell git rev-parse --short HEAD)
GOARGS			?=
GOOS			?= $(shell go env GOOS)
GOARCH			?= $(shell go env GOARCH)
WASM_DEBUG 		?= true
LINK_BIN 		?= /usr/local/bin
DIST_DIR		?= dist
DIST_OS			?= darwin windows linux
DIST_ARCH		?= arm64 amd64

EXAMPLE_DEPS 	:= dist/wanix.debug.wasm dist/wanix.min.js workbench/code extras/dist/wanix-linux.tgz test/gojs/gojscheck.wasm

export DOCKER_CMD 	?= $(shell command -v docker)

## Link/install the local Wanix command
link:
	[ -f "$(LINK_BIN)/$(NAME)" ] && rm "$(LINK_BIN)/$(NAME)" || true
	ln -fs "$(shell pwd)/.local/bin/$(NAME)" "$(LINK_BIN)/$(NAME)"
.PHONY: link

## Build Wanix runtime and command
all: js wasm cmd
.PHONY: all

## Build and run examples
examples: $(EXAMPLE_DEPS)
	@for dir in $(shell find examples -type f -iname '[mM]akefile' -exec dirname {} \;); do \
		$(MAKE) -C $$dir; \
	done
	go run ./examples/serve.go
.PHONY: examples

## Run tests against examples
test: $(EXAMPLE_DEPS)
	go test -v ./test
.PHONY: test

## Build Wanix (command and runtime) using Docker
build-docker:
	mkdir -p $(DIST_DIR)
	$(DOCKER_CMD) build \
		--build-arg GOOS=$(GOOS) \
		--build-arg GOARCH=$(GOARCH) \
		--target dist -t wanix-build -f Dockerfile .
	$(DOCKER_CMD) rm -f wanix-build > /dev/null 2>&1 || true 
	$(DOCKER_CMD) create --name wanix-build wanix-build
	$(DOCKER_CMD) cp wanix-build:/dist/. $(DIST_DIR)
	$(DOCKER_CMD) rm -f wanix-build
	mv $(DIST_DIR)/wanix .local/bin/
	ls -lah $(DIST_DIR)
	ls -lah .local/bin/wanix
.PHONY: build-docker

## Build Wanix command
cmd:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o .local/bin/wanix $(GOARGS) ./cmd/wanix
	ls -lah .local/bin/wanix
.PHONY: cmd

## Build WASM modules
wasm: $(if $(WASM_DEBUG),wasm-go,) wasm-tinygo
.PHONY: wasm

## Build WASM module using TinyGo
wasm-tinygo: wasi/worker/lib.js
	@if ! command -v tinygo >/dev/null 2>&1; then \
		echo "skipping wasm-tinygo build: tinygo not installed"; \
		exit 0; \
	fi; \
	tinygo build \
		-ldflags="-X tractor.dev/wanix.Version=$(VERSION)" \
		--no-debug \
		-target wasm \
		-o $(DIST_DIR)/wanix.wasm \
		./wasm && \
	ls -lah $(DIST_DIR)/wanix.wasm
.PHONY: wasm-tinygo

## Build WASM module using Go
wasm-go: wasi/worker/lib.js
	GOOS=js GOARCH=wasm go build \
		-ldflags="-X tractor.dev/wanix.Version=$(VERSION)" \
		-o $(DIST_DIR)/wanix.debug.wasm \
		./wasm
	ls -lah $(DIST_DIR)/wanix.debug.wasm
.PHONY: wasm-go

## Build JavaScript modules
js: node_modules
	mkdir -p $(DIST_DIR)
	go run buildjs.go
.PHONY: js


## Remove all built artifacts
clean:
	rm -rf dist
	rm -rf node_modules
	rm -f .local/bin/wanix
	rm -f wasi/worker/lib.js
	rm -f gojs/worker/lib.js
	make -C workbench clean
	make -C extras clean
	make -C rc clean
	make -C v86 clean
.PHONY: clean

go.work: toolkit-go
	go work init . ./toolkit-go

toolkit-go:
	git clone https://github.com/tractordev/toolkit-go

node_modules:
	npm install

wasi/worker/lib.js:
	make js

test/gojs/gojscheck.wasm:
	make -C test/gojs

$(DIST_DIR)/wanix.min.js:
	make js

$(DIST_DIR)/wanix.debug.wasm:
	make wasm-go

$(DIST_DIR)/wanix.wasm:
	make wasm-tinygo

workbench/code:
	make -C workbench

extras/dist/wanix-linux.tgz:
	make -C extras


## Build distribution binaries
dist: $(DIST_TARGETS)
.PHONY: dist

DIST_ASSETS 	:= $(DIST_DIR)/wanix.wasm $(DIST_DIR)/wanix.min.js $(DIST_DIR)/wanix.debug.wasm $(DIST_DIR)/wanix.handle.js $(DIST_DIR)/wanix.js
DIST_TARGETS	:= $(foreach os, $(DIST_OS), $(foreach arch, $(DIST_ARCH), $(DIST_DIR)/$(NAME)_$(VERSION)_$(os)_$(arch)))
$(DIST_TARGETS): $(DIST_DIR)/%:
	GOOS=$(word 3, $(subst _, ,$@)) \
	GOARCH=$(word 4, $(subst _, ,$@)) \
	go build -ldflags="-X main.Version=$(VERSION)" $(GOARGS) -o $@ ./cmd/wanix

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