# rc (run command for Wanix)

This directory contains a default shell for Wanix, built on top of
[`mvdan.cc/sh`](https://github.com/mvdan/sh).

The goal is a small, iteratable shell that works both:

- natively (for local development and testing), and
- in Wanix (`GOOS=js GOARCH=wasm`) using the task API.

We want to eventually implement more Plan 9 `rc` design, though not full
compatibility. This will mean replacing mvdan/sh with our own that implements
Plan 9 `rc`. Benefits we are trying to get from this:

* **Simple, orthogonal language:** small and consistent syntax, usually easier to 
read and reason about than common shell dialects.
* **Clean quoting model:** list-oriented semantics and cleaner quoting reduce many
classic Bourne-shell word-splitting surprises.
* **First-class pipeline/list thinking:** command composition is central, with 
language features that encourage plumbing small tools together.
* **Less legacy baggage*:** compared to Bash, rc has fewer historical 
compatibility constraints, making behavior feel more coherent.

We may introduce some level of compat with Bash-style shells if it
feels appropriate, but in Wanix you can always just run those shells (in a VM)
if you want that.

## Status

This is not a full Plan 9 `rc` implementation yet. It is currently a practical
POSIX-oriented shell foundation with Wanix-specific execution plumbing.

It currently does not support any pipes or redirection. Some built-ins that came
with mvdan/sh may not work, though we only list supported built-ins in help. 

We are trying to defer support for "scripting" features (functions, 
conditionals, etc) until we move off mvdan/sh. 


## Build

From `rc/`:

```sh
go build ./cmd/rc
```

Wasm build:

```sh
GOOS=js GOARCH=wasm go build ./cmd/rc
```

## Run

Command mode:

```sh
./rc -c 'echo hello'
```

Script mode:

```sh
./rc ./script.sh
```

REPL mode:

```sh
./rc
```

## Help

List shell capabilities:

```sh
help
```

Show help for a specific command or builtin:

```sh
help ls
help find
help cd
```

`help <name>` rewrites to `<name> --help`.

## Execution Model

- Native builds: external commands run via `os/exec`
- `js/wasm` builds: external commands run via Wanix task files (`#task/...`)

Path lookup supports:

- absolute paths
- relative paths
- PATH search
- subpath lookup behavior (e.g. `auth/foo` under PATH entries)

## Builtins and Bundled Commands

This shell supports `mvdan/sh` builtins (e.g. `cd`, `export`, `unset`, `type`,
etc.) plus custom `help`.

It also bundles utility commands. Current bundled commands include:

- `base64`, `cat`, `chmod`, `cp`, `env`, `find`
- `gzip`, `gzcat`, `gunzip`
- `ls`, `mkdir`, `mv`, `rm`
- `shasum`, `tar`, `touch`, `xargs`

Note: bundled commands are not shell "builtins"; they are just commands embedded
in rc similar to busybox.

## Notes on Compatibility

- Several commands were vendored/adapted under `shell/compat*` to keep behavior
  close to u-root while compiling cleanly for `GOOS=js GOARCH=wasm`.
- `env` is currently bundled and supports:
  - printing environment,
  - `-i`,
  - `KEY=VALUE` assignments for output mode.
  Command execution with modified env (`env KEY=V cmd`) is not fully implemented
  yet.
