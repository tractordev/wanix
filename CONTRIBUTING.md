# Contributing

Thank you for contributing! Here are some tips to get started quickly. If you 
run into anything in this process, be sure to submit an issue or let us know in Discord.

We want contributing and first-time experiences to be as smooth as possible.

## Building from source

### Prerequisites

You will need either Docker 20.10+ or Podman 5.5+ to build project dependencies. 

- [Docker 20.10+](https://docs.docker.com/get-docker/)
- [Podman 5.5+](https://podman.io/get-started)

You also want Go and, optionally but recommended, TinyGo.

- [Go 1.25+](https://golang.org/dl/)
- [TinyGo 0.38+](https://tinygo.org/getting-started/install/) (optional)

You will also need `make` installed.

### Our Makefile

Wanix *is* a more advanced project with a number of components working together,
so we do lean on containers and `make` to streamline the contributor experience.

We keep our `Makefile` as simple, organized, and self-documenting as possible. 
You can quickly see possible tasks with descriptions simply by running:

```sh
make
```

Below, we document all you should need to know to get started. However, don't 
be afraid to skim the `Makefile` to see what's going on.

### Dependencies

Wanix has several auxiliary components we call dependencies that are custom 
built, but don't change that often. This includes the Linux kernel, the v86
emulator, and a built-in shell image. These need to be available to build the
Wanix command. There are two ways to set these up:

#### Build the dependencies

Dependencies in particular all have their own complex build process and
toolchains, so we build them in containers. With Docker or Podman running, you 
can run this to build them all:

```sh
make deps
```

This will take a few minutes and use a lot of disk space, but typically only 
needs to be done once. Make sure that if you're running containers in a VM, like
on Mac with Docker Desktop, you allot the VM around **128GB disk storage** and
**32GB RAM**. 

It pulls a lot packages and may encounter transient network errors, so if
there is an issue, try running it again before submitting an issue.

#### Download the dependencies (soon)

> This option is not available until after the 0.3 release, tracked [here](https://github.com/tractordev/wanix/issues/212).

If building runs into problems or takes too long, you have the option to pull
prebuilt artifacts from the last release.

```sh
make deps-fetch
```

This will require `curl` and `unzip` to be available.

### Building Wanix

#### Setup symlink

Now that we have dependencies built, there is one more setup step we recommend:

```sh
make link
```

This task creates a `wanix` symlink in `/usr/local/bin` (configurable with
`LINK_BIN`) that points to where we put built binaries so that building
`wanix` is all that's needed to make it available in your PATH.

This task is totally optional and may require `sudo` on some systems.

#### Build Wanix runtime and command

```sh
make build
```

This will build the Wanix runtime (a JavaScript library and WASM module), and
then the `wanix` toolchain command binary, which embeds the runtime. This binary
is output to `.local/bin/wanix`, but if you ran `make link`, you should be able
to just run `wanix`. 

If you have TinyGo installed, the WASM module will be built with TinyGo. 
Otherwise it will use regular Go, which produces a larger binary, but has better
stacktraces for development and builds faster. 

You can force TinyGo or Go by setting `WASM_TOOLCHAIN` to either `go` or
`tinygo`. You can also specifically build the WASM module with one or the other
with `make wasm-go` and `make wasm-tinygo`. 

#### Other build options

From here you can run specific `make` tasks for specific components, just run
`make` to see what's available. For example, you can build just the `wanix` 
command (`make cmd`), just the runtime (`make runtime`), or either of the 
runtime components.

You can also build the command or the runtime inside Docker/Podman like the 
dependencies. This can be helpful if you have trouble getting Go or TinyGo
configured properly. These are tasks with the `-docker` suffix. To build both
command and runtime entirely in container, use `make build-docker`. 

Some tasks, like dependencies, are always built in Docker/Podman. This is noted
in their description with "(in Docker)".

### Running Wanix

Finally, you can run `wanix serve` to serve the default Wanix domain to access
in your browser.

## Directory Layout

TODO

---
Something missing? Let us know via GitHub issue or Discord.