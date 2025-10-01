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
VERSION 		?= v0.3-$(shell git rev-parse --short HEAD)
GOARGS			?=
GOOS			?= $(shell go env GOOS)
GOARCH			?= $(shell go env GOARCH)
WASM_DEBUG 		?= true
LINK_BIN 		?= /usr/local/bin
DIST_DIR		?= .local/dist
DIST_OS			?= darwin windows linux
DIST_ARCH		?= arm64 amd64

export DOCKER_CMD 	?= $(shell command -v podman || command -v docker)

## Link/install the local Wanix command
link:
	[ -f "$(LINK_BIN)/$(NAME)" ] && rm "$(LINK_BIN)/$(NAME)" || true
	ln -fs "$(shell pwd)/.local/bin/$(NAME)" "$(LINK_BIN)/$(NAME)"
.PHONY: link

## Build Wanix and Wanix shell
all: shell/bundle.tgz build
.PHONY: all

## Build Wanix (command and runtime)
build: runtime cmd
.PHONY: build

## Build Wanix (command and runtime) using Docker
build-docker:
	mkdir -p $(DIST_DIR)
	$(DOCKER_CMD) build --target dist -t wanix-build -f Dockerfile .
	$(DOCKER_CMD) rm -f wanix-build
	$(DOCKER_CMD) create --name wanix-build wanix-build
	$(DOCKER_CMD) cp wanix-build:/ $(DIST_DIR)/
	mv $(DIST_DIR)/wanix .local/bin/
	ls -lah $(DIST_DIR)
	ls -lah .local/bin/wanix
.PHONY: build-docker

## Build Wanix command
cmd: runtime/assets/wanix$(if $(WASM_DEBUG),.debug,).wasm runtime/assets/wanix.min.js
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o .local/bin/wanix $(GOARGS) ./cmd/wanix
	ls -lah .local/bin/wanix
.PHONY: cmd

## Build WASM and JS modules
runtime: js wasm
.PHONY: runtime

## Build WASM modules
wasm: $(if $(WASM_DEBUG),wasm-go,) wasm-tinygo
.PHONY: wasm

## Build WASM module using TinyGo
wasm-tinygo: runtime/wasi/worker/lib.js
	tinygo build -target wasm -o runtime/assets/wanix.wasm ./runtime/wasm
	ls -lah runtime/assets/wanix.wasm
.PHONY: wasm-tinygo

## Build WASM module using Go
wasm-go: runtime/wasi/worker/lib.js
	GOOS=js GOARCH=wasm go build -o runtime/assets/wanix.debug.wasm ./runtime/wasm
	ls -lah runtime/assets/wanix.debug.wasm
.PHONY: wasm-go

## Build JavaScript modules (in Docker)
js:
	$(DOCKER_CMD) build --load -t wanix-build-js --target js $(if $(wildcard runtime/assets/wanix.min.js),,--no-cache) -f Dockerfile .
	$(DOCKER_CMD) rm -f wanix-build-js
	$(DOCKER_CMD) create --name wanix-build-js wanix-build-js
	$(DOCKER_CMD) cp wanix-build-js:/build/runtime/wasi/worker/lib.js runtime/wasi/worker/lib.js
	$(DOCKER_CMD) cp wanix-build-js:/build/runtime/assets/wanix.min.js runtime/assets/wanix.min.js
	$(DOCKER_CMD) cp wanix-build-js:/build/runtime/assets/wanix.js runtime/assets/wanix.js
	$(DOCKER_CMD) cp wanix-build-js:/build/runtime/assets/wanix.handle.js runtime/assets/wanix.handle.js
.PHONY: js

## Build shell bundle for Wanix (in Docker)
bundle-shell:
	make -C shell clean bundle.tgz
.PHONY: bundle-shell

## Remove all built artifacts
clean:
	rm -f .local/bin/wanix
	rm -f runtime/assets/wanix.min.js
	rm -f runtime/assets/wanix.debug.wasm
	rm -f runtime/assets/wanix.wasm
	rm -f runtime/wasi/worker/lib.js
	make -C shell clean
.PHONY: clean


DIST_TARGETS	:= $(foreach os, $(DIST_OS), $(foreach arch, $(DIST_ARCH), $(DIST_DIR)/$(NAME)_$(VERSION)_$(os)_$(arch)))
$(DIST_TARGETS): $(DIST_DIR)/%:
	GOOS=$(word 3, $(subst _, ,$@)) \
	GOARCH=$(word 4, $(subst _, ,$@)) \
	go build -ldflags="-X main.Version=$(VERSION)" $(GOARGS) -o $@ ./cmd/wanix

## Build distribution binaries
dist: $(DIST_TARGETS)
.PHONY: dist

runtime/wasi/worker/lib.js:
	make js

runtime/assets/wanix.min.js:
	make js

runtime/assets/wanix.debug.wasm:
	make wasm-go

runtime/assets/wanix.wasm:
	make wasm-tinygo

shell/bundle.tgz:
	make bundle-shell

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