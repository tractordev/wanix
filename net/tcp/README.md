## TCP file service

This package provides a Plan 9–style `/net/tcp`-like filesystem as a Wanix file service (similar in shape to `term/`), where each TCP connection is represented as a directory of control and data files.

### Goals

- **Plan 9 familiarity**: model TCP endpoints as files (`ctl`, `data`, `status`, …).
- **Wanix conventions**: allocate new resources via a `new` file that returns a resource id.
- **Stream semantics**: `data` is a byte stream; control and status are newline-terminated text.

### Non-goals

- **Full Plan 9 parity**: only the subset needed for Wanix is required.
- **UDP, raw sockets, IP options**: out of scope for this service.

### Filesystem layout

At the service root:

- **`new`**: allocate a new connection and return its id (a short token like `1`, `2`, …) followed by `\n`.

Per-connection directories (by id):

- **`<id>/ctl`**: control channel (write-only in normal use; may be readable for debugging if useful).
- **`<id>/data`**: stream I/O for the connection.
- **`<id>/status`**: human-readable connection state.
- **`<id>/local`**: local endpoint address.
- **`<id>/remote`**: remote endpoint address.
- **`<id>/snoop`**: optional traffic tap (see “Snoop”).
- **`<id>/listen`**: present only when the connection is in “listening” mode (see “Listen”).

The service should treat unknown paths as `fs.ErrNotExist` and enforce per-file permissions appropriate to their use (typically read-only for `status/local/remote`, read-write for `data`, write-only for `ctl`).

### Address grammar

For `dial`, `bind`, and `announce`, the address operand is textual:

- **`<host>:<port>`** where `<host>` is a hostname, IPv4, or bracketed IPv6 (`[::1]:80`).
- **Port** may be numeric or a service name if resolution is supported by the runtime.
- **Empty host** is allowed in `announce` (`:8080`) to mean “all interfaces” (or the runtime default).
- **Unix socket path**: if the operand looks like a filesystem path (e.g. starts with `/` or `.`), it is treated as a Unix domain socket address.

Exact parsing rules should follow Go’s `net` conventions when possible.

### Connection lifecycle (state machine)

Each allocated connection starts in an “idle” state with no OS socket bound or connected.

- **Idle**: no local bind, not connected, not listening.
- **Bound**: has a local address reserved (via `bind`).
- **Listening**: bound and accepting inbound connections (via `announce`).
- **Connected**: active established connection (via `dial` or from `listen` accept).
- **Closed**: connection terminated; may return to Idle for reuse or remain closed (implementation choice, but must be reflected in `status`).

Errors from invalid transitions must be reported as write errors on `ctl` (e.g. dialing while already connected).

### Control file (`ctl`)

`ctl` accepts newline-terminated commands (extra whitespace ignored). Unsupported commands must fail clearly.

Supported commands:

- **`dial <addr>`**: connect to `<addr>`. On success, the connection becomes **Connected** and `remote` reflects the peer address. If a local address was not bound, one is chosen automatically.
- **`bind <addr>`**: bind the local endpoint to `<addr>` without listening. On success, the connection becomes **Bound** and `local` reflects the bound address. Binding while Connected or Listening must fail.
- **`announce <addr>`**: bind and listen on `<addr>`. If host is omitted (`:port`), listen on all interfaces (or runtime default). On success, the connection becomes **Listening** and the `listen` file appears.
- **`hangup`**: terminate an established connection (Connected → Closed). For Listening connections, this stops listening and removes `listen`. For Idle/Bound, this is a no-op or returns an error (pick one behavior and document it in `status`).

Command writes should be processed atomically per line. Partial lines (no trailing newline) may be buffered until newline or treated as an error (implementation choice).

### Data file (`data`)

`data` is the byte stream associated with the connection:

- **Connected**:
  - Reading returns bytes received from the peer.
  - Writing sends bytes to the peer.
- **Listening / Bound / Idle**:
  - Reads and writes must fail (or block with a clear error) because there is no established stream.
- **After hangup/close**:
  - Reads eventually return EOF.
  - Writes fail with a broken pipe–like error.

### Listen (`listen`)

When a connection is in **Listening** state, a `listen` file exists to accept inbound connections.

- Reading `listen` yields the id of a newly allocated connection representing the accepted connection, terminated with `\n`.
- Each successful read corresponds to one accept.
- The accepted connection starts in **Connected** state; its `local` matches the listener, and its `remote` is the peer.
- The listener connection remains in **Listening** state until `hangup` (or equivalent) stops it.

The `listen` file must not exist for non-listening connections.

### Status / local / remote

These are newline-terminated text files intended for humans and simple scripting.

- **`status`**: at minimum includes the current state (`idle|bound|listening|connected|closed`) and, when applicable, local/remote addresses. Format should be stable enough for scripts but does not need to match Plan 9 verbatim.
- **`local`**: `host:port\n` for the local endpoint when bound/connected/listening; empty or a sentinel value when idle.
- **`remote`**: `host:port\n` for the peer when connected; empty/sentinel otherwise.

### Concurrency and blocking behavior

- Multiple readers/writers on `data` may be supported; if not, attempts should error clearly.
- Blocking reads are allowed for `data` and `listen`. Cancellation should be supported via context-aware open/read where available in the surrounding runtime.

### Error handling requirements

- All failures must be surfaced as file operation errors (e.g. `write ctl`: parse error, DNS failure, connection refused, invalid state).
- Status files should reflect terminal error states when useful (e.g. “closed (ECONNRESET)”).

### Security / isolation

The service is a capability: mounting it into a process namespace grants network access. Callers should be able to omit mounting to deny TCP networking.

### Test plan (minimum)

- Allocating via `new` returns unique ids and creates a directory with the expected files.
- `announce` creates `listen`; `hangup` removes it.
- `listen` accept returns a new id in Connected state; accepted `data` I/O works end-to-end.
- Invalid `ctl` commands and invalid state transitions return deterministic errors.

### Future

Possible future additions:

- Reading `/net/tcp/n/snoop` yields a copy of traffic (either direction) in a documented framing (e.g. line-based with direction tags or length-prefixed frames).
