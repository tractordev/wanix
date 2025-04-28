# alpine exploration

The `explore/alpine` branch is actively exploring:

- [x] process to get docker hub images for wanix vms
- [x] working alpine virtual environment
- [x] networking/internet access from virtual environment
- [ ] working apk to install packages
- [ ] benchmark go compiler performance in alpine

## hubpull

There is a proof of concept utility (`hack/alpine/hubpull`) that pulls Docker
images from Docker Hub directly without Docker. Eventually this could become a
Wanix capability that creates a mount based on the layers. For now we just use
it to fetch the `i386/alpine:latest` single layer image.

## alpine

```sh
make -C hack/alpine
```

This uses `hubpull` and `fixlinks` (changes absolute symlinks to relative) and
adds the scripts in `hack/alpine/bin` to produce `hack/alpine/alpine.tgz`. 
This is embedded and served at `/alpine/alpine.tgz`. 

After the Wanix shell is loaded, in DevTools console you can run `bootAlpine()`.
This creates another VM using the `alpine.tgz` and a new terminal over the 
existing one (whole new xterm covering viewport).

This will boot with the custom init script, setting up eth0 and DHCP using the
virtual network stack that runs with `wanix serve`. It takes a moment longer 
than the normal shell. If it doesn't, run with `bootAlpine(true)` to see 
kernel output.

## apk

Currently, running `apk update` fails due to a failed rename operation, which
should also show up in the DevTools console. 

From the debug error in the console, it seems the filesystem is not resolving
to one that supports rename. It should resolve to the `#alpine` memfs that's
mounted at `web/vm/2/fsys`, but doesn't seem to. 

Goal is to update and then install a package.

## go compiler

Any package will prove apk working, but in this exploration I want to install
the Go toolchain to see how it performs. While we can compile Go to WASI and 
run it as a wasi task, it performs very poorly. Much worse than the non-WASI
build we used in 0.2. Perhaps due to the SharedArrayBuffer hack needed for WASI.

While that's a separate issue, I'm hoping an emulated x86 compiler will be fast
enough for now.