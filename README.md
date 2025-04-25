# Wanix
[![Discord](https://img.shields.io/discord/415940907729420288?label=Discord)](https://discord.gg/nQbgRjEBU4) ![GitHub Sponsors](https://img.shields.io/github/sponsors/progrium?label=Sponsors)

A virtual environment toolchain for the local-first web, inspired by Plan 9.

**📺 [Wanix: The Spirit of Plan 9 in Wasm](https://www.youtube.com/watch?v=kGBeT8lwbo0)**

### Features

* Capability-oriented microkernel architecture ("everything is a file")
* Abstract POSIX process model for generalized compute execution
* Per-process namespaces for security, isolation, and custom environments
* Built-in emulator for x86 support, Linux compatibility, and Docker-like functionality
* Runs in the browser as well as natively on Mac, Windows, and Linux
* Web: File interfaces for OPFS, DOM, web workers, and service workers
* Web: Runs WASI WebAssembly *and* x86 executables

### Example Uses

* Embed systems software and development tools in web applications
* Experiment with Plan 9 capabilities in browser or natively
* Use as foundation for a modern web-native operating system


## Try Now

Play with the stock Wanix distro at [wanix.run](https://wanix.run).


## Install Toolchain

The Wanix CLI is available for download from the [latest release](https://github.com/tractordev/wanix/releases/latest). You could also run this to install to `/usr/local/bin`:

```sh
bash -c "$(curl -sSL https://raw.githubusercontent.com/tractordev/wanix/main/install.sh)"
```

On Mac, you can install using [Homebrew](https://brew.sh/):

```sh
brew tap progrium/homebrew-taps
brew install wanix
```

If you want to build from source, see the [CONTRIBUTING.md](CONTRIBUTING.md) doc.


## Toolchain Usage

The `wanix` command has a number of subcommands in development, but the primary
command is `wanix serve`, which will serve Wanix at `http://localhost:7654`.

There is a `--listen` flag to change the port and optionally the address to listen
on. This will serve on port 6543: `wanix serve --listen :6543`


## Using the Wanix Environment

### Add Files

You can easily add files to the Wanix environment by dragging files onto the
terminal. This will put them in `/web/opfs`.

In Chrome, you can also use the `pickerfs` capability to mount a full directory
in Wanix for the duration of your session. Run `id=$(capctl new pickerfs mount)`
to bring up a directory picker. The resulting `id` can be used to get to the
mount: `cd /cap/$id/mount`.

Lastly, you can mount a tar or gzipped tar from a URL with the `tarfs` 
capability using the same process as `pickerfs` but with 
`capctl new tarfs mount <url>`.

### Run WASI

WASI Wasm executables can simply be run like running a normal executable once
added to the environment. Tested languages that can compile to Wasm
and run in Wanix include Golang, Rust, and Zig.

### Load Page in Window

Files in the root namespace can be accessed via subpaths on the domain with the
prefix `/:`, so accessing `/web/opfs/file.html` would work using 
`/:/web/opfs/file.html`. This works for any HTML elements or JS functions that
take a URL, including fetch and iframes. 

We use iframes as windows (by styling and JS), which can be created with:
```sh
id=$(domctl new iframe)
domctl body append-child $id
```
Then you can load a URL in the iframe by setting its `src` attribute:
```sh
echo src=/:/web/opfs/file.html >> /web/dom/$id/attrs
```
You can "close" a window by removing the iframe:
```sh
domctl $id remove
```

### Run JS in a Web Worker

If you have a JavaScript source file you want to run in a Web Worker, you can 
use `workerctl start <file>`, which returns a resource ID you can use under 
`/web/worker`.

You can terminate the worker with `workerctl <id> terminate`. 

### Manipulate DOM

You can currently create DOM elements using `domctl`. Run `domctl new` to see
available element types. For example, you can allocate a div element and get a
resource ID for it with `domctl new div`. DOM resources are under `/web/dom`,
including a named resource for the `body`. 

You can also append CSS styles to the page by appending to `/web/dom/style`:
```sh
echo "html { border: 8px solid lightgreen; }" >> /web/dom/style
```


## Old Demos

* [📺 Wasm I/O 2024 Demo](https://www.youtube.com/watch?v=cj8FvNM14T4)
* [📺 Mozilla Rise 25 Demo](https://www.youtube.com/watch?v=KJcd9IckJj8)

## Contributing

We are currently developing a roadmap to convey the direction we're exploring with Wanix, but this is an open and modular project that you can take and experiment with for your own purposes.

Take a look at our [issues](https://github.com/tractordev/wanix/issues) to see how you can help out. You can also ask questions and participate in [discussions](https://github.com/tractordev/wanix/discussions), however right now most discussion takes place in our [Discord](https://discord.gg/nbrwNXVvVa).

## License

MIT
