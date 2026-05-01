# JSFS Design Spec

## Overview

`jsfs` projects a JavaScript value graph into a POSIX-like filesystem interface.
Each reachable JS value becomes a node addressable by path from a root value
(for example, `globalThis`).

This allows filesystem tooling to browse and mutate JavaScript object state.

## Goals

- Represent JS values as filesystem nodes.
- Use path navigation as key selection through nested values.
- Support readable and writable files for primitive and function values.
- Support synthetic suffix views (`:obj`, `:ref`, `:json`, `:type`) that do not
  appear in directory listings.
- Provide predictable error and mutation semantics for host integration.

## Non-Goals

- Full fidelity JavaScript reflection (property descriptors, accessors, proxies).
- Security sandboxing policy (left to caller/integration).
- Cross-runtime portability guarantees beyond `syscall/js` + browser/wasm.

## Root And Path Model

- A filesystem instance is rooted at one `js.Value`.
- Paths are slash-separated key selectors, similar in spirit to JSON Pointer.
- Example: FS rooted at `globalThis` allows paths like
  `document/location/host`.
- `"."` resolves to the root value.
- Path segment semantics:
  - For objects/functions: segment is used as property key (`value[key]`).
  - For array-like values: segment is a string key; numeric strings map to index
    access in normal JS property resolution.

## Node Type Mapping

### Directory Nodes

A node is treated as a directory when:

- Value is an object or array-like and opened in directory
  context.
- `:obj` suffix is used (forced object view, see below).

Directory listing behavior:

- Default directory listing returns only direct own enumerable keys
  (equivalent to `Object.keys(value)`).
- For array-like values, keys are numeric index strings for enumerable indices.

### Primitive File Nodes

Primitive values are regular files:

- `string`
- `number`
- `bigint`
- `boolean`
- `symbol`

Primitive file read/write behavior is defined below.

### Function File Nodes

Callable values (functions/methods) are files with invocation semantics.

## Base Operations

## Read

### Primitive File

- `read` returns: `value.toString() + "\n"`.

### Function File

- Read blocks until a control line is written, executing the function, then the 
return value is readable as `value.toString() + "\n"`

## Write

### Primitive File

- Input bytes are decoded as UTF-8 string.
- `trimEnd()` is applied.
- Value is reassigned using the original primitive constructor:
  - `String(trimmed)`
  - `Number(trimmed)`
  - `BigInt(trimmed)`
  - `Boolean(trimmed)`
  - `Symbol(trimmed)` (see open issues)

### Function File

- Default mode accepts whitespace-tokenized arguments from written text.
  - Example input: `"foo 1 @foo/bar true\n"` -> `["foo", "1", "@foo/bar", "true"]`.
- Argument strings that start with `@` are resolved to the JS value for that path from `globalThis`.
- Function call is triggered when newline is observed.
- Return value becomes readable output.

## Remove

Remove is two-phase to support "unset" vs "delete from container":

1. First remove on a name sets target value to `undefined`.
2. Remove on a target already `undefined` physically removes from parent:
   - Object parent: `delete parent[key]`.
   - Array-like parent: remove by index using `splice(idx, 1)`.

## Pseudo Suffix Views

Suffix views are openable on any value but do not appear in directory listings.

Examples:

- `document/location:obj`
- `window:ref`

## `:obj` (directory view)

- Forces value to be treated as object directory.
- Includes keys across full object/value surface, not only own keys
  (prototype-inclusive behavior intended).
- Enables navigation into properties on primitives/functions/boxed values.

## `:ref` (reference assignment file)

- `read` is invalid.
- `write(data)` interprets data as a ref path:
  - Empty data -> set value to `null`.
  - Non-empty data must start with `@` and is resolved from `globalThis`.
  - Example: `"@document/location"` -> `globalThis.document.location`.

## `:json` (JSON codec view)

For non-function values:

- `read` -> `JSON.stringify(value) + "\n"`.
- `write(data)` -> `value = JSON.parse(data)`.

For function values:

- Input is JSON array argument payload.
  - Example: `["foo", 1, {"@": "document/location"}, true]`.
  - Objects with single `@` key are resolved as references using the path resolved from `globalThis`.
- Newline triggers invocation.
- Read returns `JSON.stringify(result) + "\n"`.

## `:type` (type inspection view)

- `read` -> `Object.prototype.toString.call(value).slice(8, -1) + "\n"`.
- `write` is invalid.

## Error Model

- Missing path/key resolves to `ENOENT` (`ErrNotExist`).
- Invalid operation for node/view (e.g. reading `:ref`, writing `:type`) resolves
  to `EINVAL`/invalid op.
- Function invocation exception:
  - Operation fails with `EIO`.
  - `lastError` property on the function value is set to the thrown exception.

## Consistency And Visibility

- Reads/writes operate against live JS values; no deep copy is implied.
- Directory listings reflect current value state at listing time.
- Suffix views are virtual and hidden from normal directory enumeration.

## Compatibility Notes

- Intended for `js/wasm` environments.
- Behavior depends on JS runtime property semantics and host object behavior.

## Questions / Open Issues

1. **Function read-before-first-call**: should read return EOF, empty string, or
   an explicit error when no result is buffered?
    * It should just block until there is a result
2. **Function call framing**: should newline be mandatory trigger, or should
   `Close()` also trigger invocation for partial buffered input?
    * Newline is mandatory.
3. **Tokenized argument parsing**: do we need shell-style quoting/escaping in
   default function mode (`"foo bar"` as one arg)?
    * Yes, we can use misc/shlex for this
4. **Boolean conversion semantics**: `Boolean("false")` is `true`; is this
   acceptable, or should boolean parse be strict (`true|false`)?
    * We should special case strings (normalized to lower) to become false: 
        `0`, `false`, `no`, `n`, `off`
5. **Symbol write semantics**: `Symbol(trimmed)` creates non-equal unique
   symbols every write; should symbol writes be disallowed or use registry
   (`Symbol.for`)?
    * use registry
6. **`:obj` key surface**: exact definition for prototype-inclusive listing
   (enumerable only? include non-enumerable? ordering?).
    * include non-enumerable. order is not important. probably lexicographically.
7. **`:ref` parser**: should whitespace be trimmed, and should non-`@` input be
   rejected with `EINVAL` vs treated as plain string path?
    * lets reject it if it does not have `@` and yes, trim whitespace
8. **Array delete semantics**: on second remove for `undefined`, should sparse
   arrays preserve holes or always compact with `splice`?
    * the second remove for `undefined` on array should compact with `splice`
9. **Read/write permissions by node type**: should directory nodes reject read
   and write uniformly with `EISDIR`/`EINVAL`?
    * Sure
10. **Type transitions**: if writing primitive file changes value type
    unexpectedly (e.g. number parse yields `NaN`), should this be accepted?
    * Yes, it's allowed in JS, it should be here.
11. **JSON mode errors**: should JSON parse/stringify failures map to `EINVAL`,
    `EIO`, or preserve original JS exception text?
    * Yes, EINVAL for input, EIO for output exceptions
12. **Concurrency semantics**: required guarantees for simultaneous open handles
    and interleaved writes/calls.
    * None. Though other than invoking callables as soon as "\n" is written, most mutation
      operations should happen when the file is closed.

## Appendix: Original Design Sketch 
From here on, none of this counts as authoritative, and is purely historical/archival.

javascript values are treated as file nodes

paths are key selectors from a root value. similar to json-pointer: 
- fs{globalThis} has paths like "document/location/host" 

objects/arrays are directories with file entries for every key
- key values that are objects are directories
    - only keys that are direct properties are shown
- key values that are array-like are directories
    - array-like directories have numeric index string keys
- key values that are functions are function files
- all other key value types are treated as primitive files


remove on paths
- remove for a name first sets the value to undefined
- remove for a name value that is undefined removes the value from parent:
    - deletes key on object
    - removes index from array-like, equivalent of using splice(idx,1)

primitive file types
- string
- number
- bigint
- boolean
- symbol

primitive file operations
- read => value.toString()+"\n"
- write(data) => set value to primitive type constructor(data as string with trimEnd called)

pseudo file suffixes
a couple suffix paths can be opened (but dont show up in listings) for any value,
ie: "document/location:obj" or "window:ref"

    :obj suffix directory
    treats the value like an object directory, allowing you to access fields/keys
    on primitive values, functions, etc. however instead of only hasOwnProperty
    keys the :obj suffix directory has all keys on the object/value


    :ref suffix file operations
    - read => invalid
    - write(data) => set value to given ref path string. empty data sets to null

    ref paths are filepaths starting with @ rooted with globalThis
    - "@document/location" means the value at globalThis.document.location

    :json suffix file operations
    - read => JSON.stringify(value)+"\n"
    - write(data) => set value to JSON.parse(data)

    :type suffix file operations
    - read => Object.prototype.toString.call(value).slice(8, -1)+"\n"
    - write(data) => invalid

function files
values that are callable (functions, methods) are files that can be opened, then
a string of arguments can be written ("foo 1 true" => ["foo", "1", "true"]) and 
then newline calls the function, the result being returned to read as value.toString+"\n"

:json suffix for function files
function files have :type and :ref like other files, but :json changes the format
of the input/output in the function call. opening a function file with :json suffix
means now it expects a json string of the arguments ("[\"foo\", 1, true]"), then
newline to call function, and then result json stringified can be read out.

if an exception is thrown when calling a function, we error with EIO. we also
set the "lastError" key on the function value to the caught exeption. 