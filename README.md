# WANIX
[![Discord](https://img.shields.io/discord/415940907729420288?label=Discord)](https://discord.gg/nQbgRjEBU4) ![GitHub Sponsors](https://img.shields.io/github/sponsors/progrium?label=Sponsors)

A virtual operating system kit for the local-first web, inspired by Plan 9.

### Features

* Capability-oriented microkernel architecture ("everything is a file")
* Abstract POSIX process model for generalized compute execution
* Per-process namespaces for security, isolation, and custom environments
* Built-in emulator for x86 support, Linux compatibility, and Docker-like functionality
* Runs in the browser as well as natively on Mac, Windows, and Linux
* Web: Filesystem interfaces for OPFS, DOM, web workers, and service workers
* Web: Runs WASI WebAssembly *and* x86 executables

### Example Uses

* Embed systems software and development tools in web applications
* Experiment with Plan 9 capabilities in browser or natively
* Use as foundation for a modern web-native operating system

## Install

The Wanix CLI is available for download from the [latest release](https://github.com/tractordev/wanix/releases/latest) or you can run this installer:

```sh
bash -c "$(curl -sSL https://raw.githubusercontent.com/tractordev/wanix/main/install.sh)"
```

Alternatively you can install using [Homebrew](https://brew.sh/):

```sh
brew tap progrium/homebrew-taps
brew install wanix
```

## Usage

TODO

## Old Demos

* [🎬 Wasm I/O 2024 Demo](https://www.youtube.com/watch?v=cj8FvNM14T4)
* [🎬 Mozilla Rise 25 Demo](https://www.youtube.com/watch?v=KJcd9IckJj8)

## Contributing

We are currently developing a roadmap to convey the direction we're exploring with Wanix, but this is an open and modular project that you can take and experiment with for your own purposes.

Take a look at our [issues](https://github.com/tractordev/wanix/issues) to see how you can help out. You can also ask questions and participate in [discussions](https://github.com/tractordev/wanix/discussions), however right now most discussion takes place in our [Discord](https://discord.gg/nbrwNXVvVa).

## License

MIT