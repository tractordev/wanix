# Contributing

## Build from source

First make sure you have the following tools installed:

- [Docker](https://docs.docker.com/get-docker/)
- [Go 1.23+](https://golang.org/dl/)
- [TinyGo 0.35+](https://tinygo.org/getting-started/install/)

Docker is used to build some dependency artifacts that would otherwise require
much more tooling to build. We use both big Go and TinyGo to build the project.

With a fresh checkout, run `make all` to build everything. This will take a moment
the first time to build the dependency artifacts, but unless you make changes to
the dependencies, it will be much faster on subsequent builds.

After the first build, you can run `make build` to just build the `wanix` binary.
Either way, you will end up with a native `wanix` executable in the root of the repo.

From here, typically you will run `./wanix serve` to serve Wanix in your browser.

