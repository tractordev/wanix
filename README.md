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

* Provide programmable environment for WASI to run on web pages
* Embed systems software and development tools in web applications
* Experiment with Plan 9 capabilities in browser or natively
* Use as foundation for a modern web-native operating system

## Try Now

Play with the Wanix shell bundle at [wanix.run](https://wanix.run).

* Run WASI binaries in the Wanix environment
    * Drag Rust, Zig, or Go WASI binaries onto the terminal
    * Find them under `/web/opfs` and run like any executable
* See what else you can do in [this demo](https://www.youtube.com/watch?v=kGBeT8lwbo0)

## Make your own Wanix bundle

This is a quickstart for packaging up a WASI binary to run with the Wanix Runtime
on a web page. This just scratches the surface!

#### Install the Wanix Toolchain

Download the Wanix CLI from the [latest release](https://github.com/tractordev/wanix/releases/latest)
or install with Homebrew:

```
brew install progrium/taps/wanix
```

If you want to build from source, see the [CONTRIBUTING.md](CONTRIBUTING.md) doc.

#### Create a Hello World WASI bundle

You can use any language that can compile to WASI Preview 1 format. Wanix is written in
Go, so here is a simple Go Hello World `main.go`:

```go
package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
```

Build your WASI binary to a directory called `bundle`:

```
GOOS=wasip1 GOARCH=wasm go build -o ./bundle/hello.wasm .
```

Now pack the directory into a Wanix bundle:

```
wanix bundle pack ./bundle
```

Now you should have a `bundle.tgz` file.

#### Create and serve a demo page 

Make a `demo.html` file with the source below:

```html
<html>
<body>
<script type="module">
    import { Wanix } from "./wanix.min.js";
    const w = new Wanix({bundle: "./bundle.tgz"});
    const wasiFilename = "hello.wasm";

    // a WritableStream that outputs to console.log
    const logStream = new WritableStream({
        write(chunk) {
            const text = new TextDecoder().decode(chunk);
            console.log(text.trimEnd());
        }
    });

    // create a WASI task
    const tid = (await w.readText("task/new/wasi")).trim();

    // set the cmd args to your WASI file
    await w.writeFile(`task/${tid}/cmd`, `#bundle/${wasiFilename}`);

    // get a ReadableStream of this tasks stdout
    const stdout = await w.openReadable(`task/${tid}/fd/1`);
    
    // send "start" to the tasks control file
    await w.writeFile(`task/${tid}/ctl`, "start");

    // pipe stdout to the devtools console
    stdout.pipeTo(logStream);
</script>
</body>
</html>
```

Serve the current directory using `wanix serve` on port 8080:

```
wanix serve --listen :8080
```

Browse to `http://localhost:8080/demo.html` and open DevTools console to see
your "Hello, World!"

#### What next

* Play with the default interactive shell bundle at `http://localhost:8080`
    * See what you can do in it with [this demo](https://www.youtube.com/watch?v=kGBeT8lwbo0)
* Try adding an `init.js` to your bundle. Use [this one](https://github.com/tractordev/wanix/blob/main/test/wasi/golang/init.js) for reference.
    * Now you can browse to `http://localhost:8080/?bundle=bundle.tgz` and see it run in xterm.js
* Ambitious? Make a bundle that runs an x86 Linux binary.
    * Look at the [source for our shell](https://github.com/tractordev/wanix/tree/main/shell) to see how.

## Documentation

Wanix is actively being developed, but here is what we can say so far. Wanix has
three basic components:

#### Toolchain / CLI

The `wanix` command helps you develop Wanix bundles. It also has the Wanix
runtime files embedded in it. You can export them with `wanix export <dir>`. 

#### Runtime

The Wanix runtime consists minimally of a JavaScript library (`wanix.min.js`) and
a Wasm module (`wanix.wasm`). You deploy and embed these on a page to run Wanix
bundles. The Wasm module can be thought of as the Wanix kernel, and the JavaScript 
library provides the API to the kernel.

#### Bundles

Bundles are tarballs that you make containing all the files needed for a specific 
Wanix virtual environment. Their files are exposed under the `#bundle` kernel
device. It is convention to put startup code for a bundle in `init.js`. 

### Concepts

Wanix is a Unix-like system inspired by the design of Plan 9 from Bell Labs
and its follow-up, Inferno. Both are considered the successors to Unix we never
got.

#### vfs

TODO

#### namespaces

TODO

#### tasks

TODO

#### devices / capabilities

TODO

## Contributing

We'd love your contributions! Take a look at our [issues](https://github.com/tractordev/wanix/issues) to see how you can help out. You can also ask questions and participate in [discussions](https://github.com/tractordev/wanix/discussions), however right now most discussion takes place in our [Discord](https://discord.gg/nbrwNXVvVa).

Be sure to read our [CONTRIBUTING.md](CONTRIBUTING.md) doc to get started.

## Older Demos

* [📺 Wasm I/O 2024 Demo](https://www.youtube.com/watch?v=cj8FvNM14T4)
* [📺 Mozilla Rise 25 Demo](https://www.youtube.com/watch?v=KJcd9IckJj8)

## License

MIT
