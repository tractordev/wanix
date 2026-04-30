# Wanix
[![Discord](https://img.shields.io/discord/415940907729420288?label=Discord)](https://discord.gg/nQbgRjEBU4) ![GitHub Sponsors](https://img.shields.io/github/sponsors/progrium?label=Sponsors)

A virtual environment toolkit for the web, inspired by Plan 9.

**📺 [Wanix: The Spirit of Plan 9 in Wasm](https://www.youtube.com/watch?v=kGBeT8lwbo0)**

* Run WASI and x86 programs on web pages
* Apply Plan 9 ideas in the browser
* Build a web-native operating system

### Features

* Capability-oriented microkernel architecture ("everything is a file")
* Abstract POSIX process model for generalized compute execution
* Per-process namespaces for security, isolation, and custom environments
* Built-in emulator for x86 support and Linux compatibility

## Try online demo

Play with the Wanix shell bundle at [wanix.run](https://wanix.run).


### Install the Wanix Toolchain

Download the Wanix CLI from the [latest release](https://github.com/tractordev/wanix/releases/latest)
or install with Homebrew:

```
brew install progrium/taps/wanix
```

If you want to build from source, see the [CONTRIBUTING.md](CONTRIBUTING.md) doc.

### File Services

Wanix has a number of built-in file services:

* `#task`
* `#term`
* `#vm`
* `#ramfs`
* `#pipe`
* `#signal`
* `#web`
* `#wanix`


### API Reference

For now, see [api/](api/) and [api/handle.js](api/handle.js).

## Contributing

We'd love your contributions! Take a look at our [issues](https://github.com/tractordev/wanix/issues) to see how you can help out. You can also ask questions and participate in [discussions](https://github.com/tractordev/wanix/discussions), however right now most discussion takes place in our [Discord](https://discord.gg/nbrwNXVvVa).

Be sure to read our [CONTRIBUTING.md](CONTRIBUTING.md) doc to get started.

## Older Demos

* [📺 Wasm I/O 2024 Demo](https://www.youtube.com/watch?v=cj8FvNM14T4)
* [📺 Mozilla Rise 25 Demo](https://www.youtube.com/watch?v=KJcd9IckJj8)

## License

MIT
