# Wanix
[![Discord](https://img.shields.io/discord/415940907729420288?label=Discord)](https://discord.gg/nQbgRjEBU4) ![GitHub Sponsors](https://img.shields.io/github/sponsors/progrium?label=Sponsors)

Wanix is an embeddable runtime that brings a Unix-like environment to the browser. Declare a `<wanix-namespace>`, bind files and archives into it, run Wasm and JavaScript tasks, boot Linux in an x86 emulator, and wire up terminals and a VS Code workbench, all from HTML.

```html
<wanix-namespace>
  <wanix-bind type="file" dst="helloworld.wasm" src="./helloworld.wasm"></wanix-bind>
  <wanix-task cmd="helloworld.wasm" term start></wanix-task>
  <wanix-term path="#task/1/term"></wanix-term>
</wanix-namespace>
```

- **Everything is a file.** Processes, terminals, VMs, browser APIs, and storage are exposed through a unified namespace you compose with binds. The same idea as Plan 9, with improvements, in the browser.
- **Composable environments.** Layer tar archives, fetch remote files, write inline scripts, and union directories to build exactly the filesystem your app needs.
- **Pluggable compute.** Run Go/TinyGo (`gojs`), WASI Wasm, JavaScript workers, and x86 Linux (via v86) as tasks in the same namespace.
- **Isolation by design.** Each task gets its own namespace. VMs export a guest namespace. Import remote namespaces over 9P.
- **Browser-native integration.** OPFS persistence, DOM control, web workers, service workers, and fetch are first-class via the `#web` namespace.
- **No backend required.** The runtime (`wanix.min.js` + `wanix.wasm`) runs entirely client-side. Host static assets on any CDN.
- **Progressive complexity.** Start with a single Wasm binary and terminal. Add Linux VMs, a full IDE workbench, or cross-origin federation when you need them.

## Use Cases

- **In-browser dev environments**: edit/run code without a remote container.
- **Interactive demos and tutorials**: embed reproducible sandboxes in docs, blog posts, and courseware.
- **Local-first apps**: persist user data in OPFS, run logic in workers, and build your own platform.
- **Agent sandboxes**: utilize browser sandboxing to isolate an agent environment you construct.
- **Personal compute**: build your own computing environment / operating system.

## Quick Start (CDN)

```html
<!DOCTYPE html>
<html>
<head>
  <script type="module"
    src="https://cdn.jsdelivr.net/npm/wanix@0.4.0-alpha8/dist/wanix.min.js">
  </script>
</head>
<body style="height: 100vh; margin: 0;">

  <wanix-namespace>
    <!-- bind alloc ramfs to namespace root -->
    <wanix-bind dst="." src="#ramfs/new"></wanix-bind>
    <!-- bind inline file into namespace -->
    <wanix-bind type="file" dst="hello.sh" perm="0755">
      echo "Hello from Wanix!"
    </wanix-bind>
    <!-- bind wasm executable from url -->
    <wanix-bind type="file" dst="rc.wasm"
      src="https://cdn.jsdelivr.net/npm/wanix-extras@0.4.0-rc1/dist/rc.wasm">
    </wanix-bind>
    <!-- declare a task that will autostart -->
    <wanix-task id="shell" cmd="rc.wasm -c hello.sh" term start></wanix-task>
    <!-- show a terminal wired up to the task -->
    <wanix-term path="#task/shell/term"></wanix-term>
  </wanix-namespace>

</body>
</html>
```

### JavaScript API

After `<wanix-namespace>` fires a `ready` event, use the filesystem handle:

```html
<script type="module">
  const sys = document.querySelector('wanix-namespace');
  sys.addEventListener('ready', async () => {
    console.log(await sys.root.readDir('.'));
    await sys.root.writeFile('note.txt', 'saved from JS');
  });
</script>
```

Full API: [api/handle.js](api/handle.js).


## Elements Reference

Elements let you compose a Wanix system in HTML. The only visual elements are
`<wanix-term>` and `<wanix-workbench>`.

### `<wanix-namespace>`

The root namespace and Wasm kernel. All other elements live inside it, reference it with `for`, or have an implicit namespace because they are the
root element. Any element other than `<wanix-bind>` can be used as the root element instead of `<wanix-namespace>` and will additionally take its
attributes. 

| Attribute | Description |
|-----------|-------------|
| `wasm` | URL to the Wanix Wasm module. Defaults to `./wanix.wasm` next to the bundle. |
| `debug` | Enable DevTools helpers and verbose logging. |
| `id` | System identifier used for cross-origin import/export. |
| `allow-origins` | Space-separated origins allowed to import this system via `postMessage` (use `*` to allow all). Requires `id`. |

Events: `ready` (namespace usable), `error` (load failure).

### `<wanix-bind>`

Mount a source into the namespace at `dst`. 

| Attribute | Description |
|-----------|-------------|
| `dst` | Destination path. `.` is the namespace root. Paths *do not* start with `/`. |
| `src` | Source path or URL. System paths use `#` prefix (e.g. `#ramfs`, `#web/opfs`). |
| `type` | Bind type (see below). Default: `ns`. |
| `perm` | File permission mode for `file` binds. Default: `0644`. |
| `union` | Union mode when binding to an existing directory. Default: `after`. |

#### Bind types

| Type | Behavior |
|------|----------|
| `ns` | Bind another namespace path (default). |
| `file` | Write element text content (or fetched URL if `src` is set) to `dst`. |
| `archive` | Fetch a `.tar` or `.tar.gz` and mount as a directory tree. |
| `import` | Import a remote Wanix namespace via WebSocket (`ws://` / `wss://`) or iframe + 9P (`src` URL with `#system-id`). |

### `<wanix-task>`

Allocate and run a task, which is shaped like a process (args, env, stdio, ...) and executed by
a task driver. Tasks run in their own namespace, by default inheriting the current/root namespace.

| Attribute | Description |
|-----------|-------------|
| `cmd` | Command line to run. |
| `type` | Task driver: `auto`, `gojs`, `wasi`, `js`, etc. Default: `auto`. |
| `role` | Semantic role. Use `shell` for workbench shell templates. |
| `id` / `alias` | Optional name for referencing the task at `#task/<id>/…`. |
| `env` | Environment variables, space-separated `KEY=VALUE` pairs (use quotes for values with spaces). |
| `wd` | Working directory within the task namespace. |
| `fsys` | Base filesystem path for the task namespace. |
| `stdin` / `stdout` / `stderr` | Namespace paths for I/O redirection. |
| `term` | Allocate a terminal device for this task. |
| `start` | Start the task automatically when the system is ready. |
| `for` | ID of a `<wanix-namespace>` to attach to (instead of being a direct child). |

Terminal path after allocation: `#task/<id>/term` (or `#task/<rid>/term` without alias).

#### Task drivers

| Type | Behavior |
|------|----------|
| `auto` | Determine automatically (default). |
| `js` | Run plain JavaScript as a task. |
| `gojs` | Run Wasm compiled by Go using `GOOS=js GOARCH=wasm`. |
| `wasi` | Run any Wasm compiled using `wasi`/`wasip1`. |


### `<wanix-vm>`

Allocate and run a virtual machine.

| Attribute | Description |
|-----------|-------------|
| `type` | VM backend. Default: `v86`. |
| `id` / `alias` | Optional name. Terminal at `#vm/<id>/term`. |
| `fsys` | Root filesystem path in the namespace. |
| `term` | Allocate a terminal for serial console I/O. |
| `start` | Boot automatically when ready. |
| `append` | Kernel command line additions. |
| `export` | Host-export device (e.g. `ttyS0`, `hvc1`) for guest ↔ host bridging. |
| `mem` | RAM size (e.g. `512M`, `1G`). |
| `boot`, `bios`, `netdev`, … | Additional QEMU-style flags mapped from attribute names. |

Using `<wanix-vm>` requires a VM backend to be loaded using bind to `#vm/<type>`:

```html
<wanix-bind dst="#vm/v86" type="archive" src="https://cdn.jsdelivr.net/npm/wanix-extras@0.4.0-rc1/dist/v86.tgz"></wanix-bind>
```

### `<wanix-term>`

Render an [xterm.js](https://xtermjs.org/) terminal connected to a Wanix terminal device.

| Attribute | Description |
|-----------|-------------|
| `path` | Terminal device path (e.g. `#term/1`, `#task/shell/term`, `#vm/1/term`). |
| `raw` | Raw mode — no local line editing; bytes pass through directly. Use for VM serial consoles. |
| `for` | ID of a `<wanix-namespace>` to attach to. |

Style the element with `height: 100%` (and flex layout on parents) for full-page terminals.

### `<wanix-workbench>`

Embed a VS Code web workbench backed by the Wanix filesystem.

| Attribute | Description |
|-----------|-------------|
| `assets` | URL prefix for workbench static assets (built with `make -C workbench`). |
| `wd` | Workspace folder path in the namespace (e.g. `root`, `.`). |
| `open` | Space-separated file paths to open on startup. |
| `term` | Enable integrated terminal panel. |
| `raw` | Raw terminal mode for integrated terminal. |
| `sidebar` | Initial sidebar state: `default`, `hidden`, `never` (hidden even if user previously opened), or `always` (open even if user previously closed). |
| `panel` | Initial panel state: `default`, `hidden`, `never`, `always`, `max` (seed maximized), or `always-max` (maximized even if user previously restored). |
| `fresh` | Clear stored workbench UI/profile state before startup (do not restore previous layout). |
| `welcome` | Show welcome page on startup. |
| `debug` | Verbose workbench logging. |
| `task-ns`, `term-ns` | Override task/terminal namespace paths (e.g. for VM guest shells). |

Include a child `<wanix-task role="shell" …>` as the shell template.

## System Namespace

These `#` paths are provided by the kernel and can be bound into your namespace:

| Path | Description |
|------|-------------|
| `#task` | Process control and task namespaces. |
| `#term` | Terminal devices. |
| `#vm` | Virtual machine control. |
| `#ramfs` | In-memory filesystem (cloned per bind). |
| `#pipe` | Pipe pairs (cloned per bind). |
| `#signal` | Signal devices (cloned per bind). |
| `#web` | Browser integration — OPFS (`#web/opfs`), DOM, workers, caches, etc. |
| `#wanix` | Internal Wanix devices. |


## Recipes

### Minimal Wasm terminal

Run a Go/TinyGo Wasm binary with a terminal:

```html
<wanix-namespace>
  <wanix-bind type="file" dst="app.wasm" src="https://example.com/app.wasm"></wanix-bind>
  <wanix-task id="app" cmd="app.wasm" term start></wanix-task>
  <wanix-term path="#task/app/term"></wanix-term>
</wanix-namespace>
```

See [examples/basic-terminal.html](examples/basic-terminal.html).

### Writable namespace with inline files

Create a virtual filesystem to use via JS:

```html
<wanix-namespace>
  <wanix-bind dst="." src="#ramfs/new"></wanix-bind>
  <wanix-bind dst="greeting.txt" type="file" perm="0644">
    Hello, world!
  </wanix-bind>
</wanix-namespace>
```

See [examples/basic-namespace.html](examples/basic-namespace.html).

### JavaScript worker task

Run inline JS in a Wanix task:

```html
<wanix-namespace>
  <wanix-bind dst="." src="#ramfs/new"></wanix-bind>
  <wanix-bind dst="task.js" type="file" perm="0766">
    console.log("JS task running!");
  </wanix-bind>
  <wanix-task cmd="task.js" start></wanix-task>
</wanix-namespace>
```

See [examples/task-js.html](examples/task-js.html).

### Layered root filesystem

Stack archives and overlay individual files — later binds win:

```html
<wanix-namespace>
  <wanix-bind type="archive" dst="root"
    src="https://example.com/base-rootfs.tar.gz"></wanix-bind>
  <wanix-bind type="archive" dst="root"
    src="https://example.com/overlay.tar.gz"></wanix-bind>
  <wanix-bind type="file" dst="root/boot/bzImage"
    src="https://example.com/custom-kernel"></wanix-bind>
</wanix-namespace>
```

See [examples/bind-types.html](examples/bind-types.html).

### Boot Linux in v86

```html
<wanix-namespace>
  <wanix-bind dst="." type="archive"
    src="https://cdn.jsdelivr.net/npm/wanix-extras@0.4.0-rc1/dist/wanix-linux.tgz">
  </wanix-bind>
  <wanix-bind dst="#vm/v86" type="archive"
    src="https://cdn.jsdelivr.net/npm/wanix-extras@0.4.0-rc1/dist/v86.tgz">
  </wanix-bind>
  <wanix-vm export="ttyS0" mem="1G" term start></wanix-vm>
  <wanix-term path="#vm/1/term" raw></wanix-term>
</wanix-namespace>
```

See [examples/basic-vm.html](examples/basic-vm.html).

### VS Code workbench with rc shell

Host workbench assets locally (`make -C workbench`), then:

```html
<wanix-namespace debug>
  <wanix-bind type="archive" dst="root"
    src="https://cdn.jsdelivr.net/npm/wanix-extras@0.4.0-rc1/dist/wanix-linux.tgz">
  </wanix-bind>
  <wanix-bind type="fetch" dst="rc.wasm"
    src="https://cdn.jsdelivr.net/npm/wanix-extras@0.4.0-rc1/dist/rc.wasm">
  </wanix-bind>
  <wanix-workbench assets="/workbench" term>
    <wanix-task role="shell" cmd="rc.wasm"></wanix-task>
  </wanix-workbench>
</wanix-namespace>
```

See [examples/basic-workbench.html](examples/basic-workbench.html).

### Workbench with VM guest shell

Edit files on the host namespace while running shells inside a Linux VM:

```html
<wanix-namespace debug>
  <wanix-bind dst="." type="archive" src="/assets/wanix-linux.tgz"></wanix-bind>
  <wanix-bind dst="#vm/v86" type="archive" src="/assets/v86.tgz"></wanix-bind>
  <wanix-vm export="ttyS0" start></wanix-vm>
  <wanix-workbench assets="/workbench"
    task-ns="#vm/1/guest/#task"
    term-ns="#vm/1/guest/#term"
    raw term>
    <wanix-task role="shell" cmd="bin/sh"></wanix-task>
  </wanix-workbench>
</wanix-namespace>
```

See [examples/vm-workbench.html](examples/vm-workbench.html).

### OPFS-backed persistent editor

Persist files in the browser with Origin Private File System:

```html
<wanix-namespace>
  <wanix-bind dst="." src="#web/opfs"></wanix-bind>
  <wanix-bind dst="main.js" type="file" perm="0644">
    export default function() { return 42; }
  </wanix-bind>
  <wanix-workbench open="main.js" assets="/workbench"></wanix-workbench>
</wanix-namespace>
```

### Export and import namespaces

Export a namespace from one page:

```html
<wanix-namespace id="main" allow-origins="*">
  <wanix-bind dst="." src="#ramfs/new"></wanix-bind>
  <wanix-bind dst="shared.txt" type="file">shared data</wanix-bind>
</wanix-namespace>
```

Import it from another page:

```html
<wanix-namespace>
  <wanix-bind type="import" dst="remote"
    src="https://other.example/app.html#main"></wanix-bind>
  <wanix-task id="repl" cmd="rc.wasm" term start></wanix-task>
  <wanix-term path="#task/repl/term"></wanix-term>
</wanix-namespace>
```

Import over WebSocket 9P:

```html
<wanix-bind type="import" dst="home" src="wss://example.com/9p"></wanix-bind>
```

See [examples/example-export.html](examples/example-export.html) and [examples/bind-import.html](examples/bind-import.html).

### Remote VM in a local workbench

Import a VM running on another origin and attach a workbench to its guest namespace:

```html
<wanix-namespace debug>
  <wanix-bind type="import" dst="remote"
    src="https://vm-host.example/linux.html#linux"></wanix-bind>
  <wanix-workbench assets="/workbench"
    task-ns="remote/vm/1/guest/#task"
    term-ns="remote/vm/1/guest/#term"
    raw term>
    <wanix-task role="shell" cmd="bin/sh"></wanix-task>
  </wanix-workbench>
</wanix-namespace>
```

See [examples/import-workbench.html](examples/import-workbench.html).

### More examples

See all [examples](examples).


## Local Development

```sh
make build          # build runtime + wanix CLI
make examples       # serve examples at http://localhost:7070/examples
make                # show all make tasks
```

Using `make examples` will build extras the first time, and will need
Docker running to succeed. Podman users will need to set `DOCKER_CMD=podman`.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full build guide.

## Contributing

We'd love your contributions! Take a look at our [issues](https://github.com/tractordev/wanix/issues) to see how you can help out. You can also ask questions and participate in [discussions](https://github.com/tractordev/wanix/discussions), however right now most discussion takes place in our [Discord](https://discord.gg/nQbgRjEBU4).

Be sure to read our [CONTRIBUTING.md](CONTRIBUTING.md) doc to get started.

## AI Disclosure

There are components of Wanix that have been written with AI assistance. 
However, we will not accept "vibe coded" PRs, which is to say a human needs to 
know how the PR works and be responsible for it.

## Older Demos

* [📺 2025 Wanix 0.3 Preview Demo](https://www.youtube.com/watch?v=kGBeT8lwbo0)
* [📺 2024 Wasm I/O Demo](https://www.youtube.com/watch?v=cj8FvNM14T4)
* [📺 2023 Mozilla Rise Demo](https://www.youtube.com/watch?v=KJcd9IckJj8)

## License

MIT
