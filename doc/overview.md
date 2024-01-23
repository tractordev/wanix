## How it works

This is a brief overview of the high level components used in the Wanix system.

#### Dev server

The dev server serves an empty HTML page that just includes the bootloader, which is a single JavaScript file used to bootstrap a Wanix environment. In development, the dev server serves the Wanix project directory to the environment, but a production bundle of the bootloader would include all files needed to bootstrap an environment. 

#### Bootloader

The bootloader JS file has several functions to bootstrap a Wanix environment and start the Wanix kernel. In a production bundle it would include compressed files and the code to inflate them into a pseudo filesystem called initfs. It also registers itself as a service worker to intercept requests, so some of its code will only run when loaded as a service worker. As a service worker it either handles requests to access initfs files, or hands requests off to the gateway component of the kernel once the kernel is running.

Most importantly, it starts the kernel in a web worker using an abstraction called a task. A task is a thin layer around a web worker made to run WebAssembly that sets it up with a Duplex pipe for two-way RPC communication. This task primitive is also used by the kernel for processes. Finally, it sets up a "pagehost API", allowing the kernel to perform operations on the host page. 

#### Kernel

The kernel is a WebAssembly module written in Go made of several components. There is an `fs` component that sets up and manages the root filesystem and other filesystems that can be mounted on it. There is a `web` component that sets up the host page with iframes of loaded "apps", initially just the default console. The `web` component also provides a gateway service for the service worker to access the root filesystem, which can also transform files on the fly, such as transpiling TS and JSX files. There is a `tty` service that manages terminal sessions, for example for the default console. Finally there is a `proc` service that manages processes, which are simply tasks. So processes are WebAssembly modules running in their own web worker with a two way connection to the kernel. 

#### Tasks

Process tasks expose an API to the kernel to start them, connect standard IO, and anything else that needs to be done from inside the web worker. The API the exposed to the task from the kernel is for the Wanix equivalent of syscalls. This API is defined by the kernel components, such as `proc` to spawn subprocesses, and `fs` to interact with the filesystem. 

Tasks are currently designed specifically to run WebAssembly modules written in Go for the "js" runtime (GOOS), but should eventually run any WebAssembly conforming to WASI. One possible issue in the way of supporting WASI is that the WASI API is synchronous and our syscalls must be asynchronous since they happen across web workers. In the meantime, since Wanix is written in Go and we only have a Go compiler in the Wanix environment, we are aligning Wanix around Go regardless of support for other toolchains. Go is to Wanix what C is to Unix. 

#### Initial Compiler (build)

The compiler inside Wanix, `build`, is a subset of the Go toolchain, namely the `compile` and `link` tools. This is because the full Go binary doesn't function in the Wanix environment yet. The `build` module also contains precompiled package archives for the standard library. This makes it more self-contained, eliminating the need for a GOROOT. But also because the standard library hasn't been able to be compiled within Wanix yet.

This compiler is also limited to linking against packages built into its precompiled package archive, which is currently the standard library. Although other packages can be added to this, it's currently easier to compile outside of Wanix and bring executables in via initfs. The other problem is 3rd party packages must be downloaded from other domains, and from the browser CORS typically prevents this. At some point we can tunnel downloads through the dev server, but it's unlikely we can support 3rd party packages without the dev server unless somebody wants to run a public proxy service. However, for security and change management reasons, we are okay with this constraint. Either you explicitly add precompiled package archives, or you must be in dev mode. 

Despite this limitation, the `build` compiler is otherwise a full Go compiler and is even capable of cross compilation.

#### Shell

The kernel initially launches a console on the host page connected to a tty running `shell`. Like `build`, this is a fully independent and standalone program. It is not an existing shell, nor is it POSIX compliant. However, it is POSIX-ish with the intention of approaching POSIX compliance but also exploring new ideas for this new kind of environment. It uses kernel syscalls to launch processes like any other task. We want to make sure you can build and rebuild the shell from inside Wanix to dogfood and showcase the self-modifiable nature of the Wanix environment. This `shell` can also run shell scripts.
