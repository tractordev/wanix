# Wanix
[![Discord](https://img.shields.io/discord/415940907729420288?label=Discord)](https://discord.gg/nQbgRjEBU4) ![GitHub Sponsors](https://img.shields.io/github/sponsors/progrium?label=Sponsors)

A virtual environment runtime for the web, inspired by Plan 9.

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

* Run WASI binaries in the Wanix environment
    * Drag Rust, Zig, or Go WASI binaries onto the terminal
    * Find them under `/web/opfs` and run like any executable
* See what else you can do in [this demo](https://www.youtube.com/watch?v=kGBeT8lwbo0)

## Make your own Wanix bundle

This is how to package up a WASI binary to run with the Wanix Runtime
on a web page.

### Install the Wanix Toolchain

Download the Wanix CLI from the [latest release](https://github.com/tractordev/wanix/releases/latest)
or install with Homebrew:

```
brew install progrium/taps/wanix
```

If you want to build from source, see the [CONTRIBUTING.md](CONTRIBUTING.md) doc.

### Create a Hello World WASI bundle

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
go mod init hello
GOOS=wasip1 GOARCH=wasm go build -o ./bundle/hello.wasm .
```

Now pack the directory into a Wanix bundle:

```
wanix bundle pack ./bundle
```

Now you should have a `bundle.tgz` file.

### Create and serve a demo page 

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

### What next

* Play with the default interactive shell bundle at `http://localhost:8080`
    * See what you can do in it with [this demo](https://www.youtube.com/watch?v=kGBeT8lwbo0)
* Try adding an `init.js` to your bundle. Use [this one](https://github.com/tractordev/wanix/blob/main/test/wasi/golang/init.js) for reference.
    * Now you can browse to `http://localhost:8080/?bundle=bundle.tgz` and see it run in xterm.js
* Ambitious? Make a bundle that runs an x86 Linux binary.
    * Look at the [source for our shell](https://github.com/tractordev/wanix/tree/main/shell) to see how.

## Documentation

Wanix is a Unix-like system inspired by the design of Plan 9 from Bell Labs
and its follow-up, Inferno. Both are considered the successors to Unix that we never
got. However, Wanix is not (yet) a bootable operating system. It's an embeddable
runtime that is platform agnostic. It's small enough for embedding simple programs,
but generative enough to be a platform for new computing environments. 

### Toolchain / CLI

The `wanix` command helps you develop Wanix bundles. It also has the Wanix
runtime files embedded in it. You can export them with `wanix export <dir>`. 

### Runtime

The Wanix runtime consists minimally of a JavaScript library (`wanix.min.js`) and
a Wasm module (`wanix.wasm`). You deploy and embed these on a page to run Wanix
bundles. The Wasm module can be thought of as the Wanix kernel, and the JavaScript 
library provides an API to that kernel.

### Bundles

Bundles are tarballs that you make containing all the files needed for a specific 
Wanix virtual environment. Their files are exposed under the `#bundle` kernel
device. It is convention to put startup code for a bundle in `init.js`. They can
be as simple as containing a single WASI executable, or as complex as containing
multiple system images.

### Abstractions (the magic)

Similar to Unix-like systems, the key abstractions in Wanix are processes and 
filesystems. In fact, even more than Plan 9, Wanix focuses primarily on these 
as primitives. Luckily, Plan 9 was thoughtfully designed to get the most out of 
them, particularly filesystems. By emulating the design of Plan 9, Wanix 
remains incredibly simple while conserving immense power.

While Wanix could be seen as "just a VFS" for running Wasm, the magic is in 
subtle design decisions that are easiest to explain through their lineage.

#### History of the Virtual Filesystem

Unix pioneered the modern hierarchical filesystem as a way to organize data on
multiple disks into a single namespace. However, it quickly became obvious the 
API was more generally useful than a way to read and write files on disk. Really, 
a filesystem is a name-based multiplexer and lookup system for any stream of 
bytes. Working with files, network connections, and even hardware ultimately 
comes down to streams of bytes.

Unix leaned into this idea by modeling hardware devices and some system services
as virtual files alongside "real" files in a unified hierarchy of names. This 
meant the same API and namespace could be used for more than data on disk, and 
all the higher level file commands and tooling, including the shell, could 
be used to interact with both files and devices. This was the origin
of the "device namespace" we know as `/dev`.

When the Unix authors got a chance to design a new operating system for 
themselves as programmers and power users, they realized they could both reduce
the number of syscalls and expose more power by modeling even more through the
filesystem. As a result, Plan 9 makes the filesystem abstraction itself a 
first-class citizen, where "everything is a filesystem" and files are really
treated as named streams of bytes.

#### Namespaces and Capabilities

In Wanix, instead of a single virtual filesystem, every process has its own 
virtual filesystem called a namespace. Specific file services are mounted into
a namespace, which is all the process in that namespace can see or interact 
with. It's similar to chroots, jails, or containers, except you get them by default for
every process. For convenience, though, the default behavior is to start with
a copy of the parent process namespace.

Some file services mounted into a namespace are for data storage, but
many file services are more of an API. In other words, they're capabilities. By
making the namespace filesystem the only way to interact with system
functionality, and making each process have its own namespace, there is always
control over what any process can or cannot do. 

Not only does this make everything work through a single, famililar API, it's also
the most ubiquitous API. Every major platform has and can work with filesystems.
Every language has filesystem operations in their standard library. And there are 
a number of protocols for network filesystems, including 9P from Plan 9. 

Even if a platform doesn't expose capabilities the way Wanix does, they still 
have the ability to mount and interact with them. While working with Wanix 
(or Plan 9), consider the possibilities of any namespace, file service, or 
capability being mounted on Linux, Windows, or Mac. Wanix capabilities are 
effectively universal.

So even if the main use for Wanix namespaces is giving a WASI program a virtual
Unix-like filesystem to work with, it can do that just fine. While also having the
potential for so much more, without being much more.

#### Tasks

The other core abstraction Wanix provides is called a task. This is the equivalent
of a process and even has the same shape of a POSIX process. They're called tasks
instead so they can co-exist with processes.

The main difference from processes is tasks are more general. They're effectively 
virtual processes since they don't need to be a POSIX process. They could be 
backed by a web worker, or a virtual machine, or a remote job. Whatever they 
really are, they have the "shape" of a POSIX process:

* arguments
* environment variables
* a working directory
* standard IO (stdin, stdout, stderr)
* an exit code
* a filesystem (namespace)

Tasks in Wanix are created and managed via file service. Where Plan 9 syscalls fall into 
two categories, filesystem and processes, Wanix only has the one. I guess it
really is *just* ("just") a VFS. 

### File Services

Wanix has a number of built-in file services. They are:

* `#task`
* `#cap`
* `#bundle`
* `#console`
* ...

(todo, document more and in more detail)

### API Reference

For now, see [runtime/fs.js](runtime/fs.js) and [runtime/wanix.js](runtime/wanix.js).

## Contributing

We'd love your contributions! Take a look at our [issues](https://github.com/tractordev/wanix/issues) to see how you can help out. You can also ask questions and participate in [discussions](https://github.com/tractordev/wanix/discussions), however right now most discussion takes place in our [Discord](https://discord.gg/nbrwNXVvVa).

Be sure to read our [CONTRIBUTING.md](CONTRIBUTING.md) doc to get started.

## Older Demos

* [📺 Wasm I/O 2024 Demo](https://www.youtube.com/watch?v=cj8FvNM14T4)
* [📺 Mozilla Rise 25 Demo](https://www.youtube.com/watch?v=KJcd9IckJj8)

## License

MIT
