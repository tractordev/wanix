# Contributing

## Build from source

### Prerequisites

At minimum you will need Docker 20.10.0 or later, which is used to build the
project dependencies. Then we recommend building Wanix directly using the Go 
and TinyGo toolchains. If you have trouble with that you can use Docker to build
but you will still need at least Go installed.

- [Docker 20.10+](https://docs.docker.com/get-docker/)
- [Go 1.23+](https://golang.org/dl/)
- [TinyGo 0.35+](https://tinygo.org/getting-started/install/) (optional)

### Building dependencies

```sh
make deps
```

This will take a moment and a lot of disk space, but will only need to be done 
once. It pulls a lot packages and may encounter transient network errors, so if
there is an issue, try running it again before submitting an issue.

### Building Wanix

```sh
make build
```

This will build the `wanix` binary and put it in the project root. By default it
uses TinyGo to build the WASM module and "big" Go to build the native executable.

#### Building without TinyGo

You can build the executable *and* WASM module with Go, which will be faster
and have better stack traces, but will be larger:

```sh
make wasm-go wanix
```

#### Building with Docker

If you have trouble building with Go, you can use Docker to build Wanix, however
it will be built without the `console` subcommand:

```sh
make docker
```

This will build a local Docker image called `wanix` that has the project and
`wanix` binary built. The `make docker` task will also copy the `wanix` binary
into your project root. By default it will be built for your host platform.


### Running Wanix

From here, typically you will run `./wanix serve` to serve Wanix to your browser.

