## Ether file service

This package provides a Plan 9–style `/net/ether0` filesystem as a Wanix file service. It wraps a [`vnet.VirtualNetwork`](https://github.com/progrium/go-netstack) (a userspace virtual switch + gateway with DHCP/DNS/NAT). Each conversation is a virtual NIC plugged into that switch: allocating a conversation starts a `vn.AcceptQemu(ctx, conn)` session, and the conversation's `data` file carries raw Ethernet frames to/from it.

### Usage

```go
cfg := vnet.Configuration{MTU: 1500, Subnet: "192.168.127.0/24", GatewayIP: "192.168.127.1", GatewayMacAddress: "5a:94:ef:e4:0c:dd"}
vn, _ := vnet.New(&cfg)
ether0 := ether.New(vn, cfg)
```

### Goals

- **Plan 9 familiarity**: mirror the shape of `ether(3)` (`clone`, `<n>/ctl`, `<n>/data`, `<n>/type`, `<n>/stats`).
- **Frame semantics**: one `write` on `data` sends exactly one Ethernet frame; one `read` returns exactly one frame.
- **Thin wrapper**: all switching, ARP, DHCP, DNS and NAT is done by `vnet`; this service only adapts files to `AcceptQemu`.

### Non-goals

- **Full `ether(3)` parity**: no `promiscuous`, `bridge`, `headersonly`, multicast (`addmulti`/`remmulti`), or `ifstats`.
- **Ethertype demultiplexing**: every conversation sees all frames `vnet` delivers to it (as if `connect -1`).
- **Multiple interfaces**: one service instance wraps one `VirtualNetwork` and is mounted as `ether0`.

### Filesystem layout


echo "hangup" > /net/ether0/$id/ctl 
/net/ether0/$id/data

At the service root:

- **`clone`**: opening it allocates a new conversation `n` and returns an open handle to `<n>/ctl` (see “Conversation lifecycle”). It is never read or written as a file of its own.
- **`addr`**: the gateway MAC address as 12 lowercase hex digits, no separators (e.g. `5a94efe40cdd`), followed by `\n`.
- **`stats`**: read-only text describing the network (see “Stats”).

Per-conversation directories (by id, `0`, `1`, …):

- **`<n>/ctl`**: control file (see “Control file”). Reading returns the id followed by `\n`, as in Plan 9.
- **`<n>/data`**: read-write Ethernet frame I/O.
- **`<n>/type`**: read-only; always `-1\n` (all ethertypes).
- **`<n>/stats`**: read-only per-conversation counters.

Unknown paths return `fs.ErrNotExist`. Permissions: `ctl`, `data` read-write; `addr`, `type`, `stats` read-only. `clone` is opened read-write (to match its `ctl` handle).

### Conversation lifecycle

Like Plan 9, a conversation lives as long as it has open files.

1. **Allocate**: opening `clone` reserves the next id `n`, creates `<n>/`, and returns a handle that behaves exactly like an open `<n>/ctl` (its `Stat` reports the name `ctl`). The client reads it to learn `n`, and may write `connect` to it.
2. **Start**: at allocation, the service creates an in-memory duplex pipe and runs `vn.AcceptQemu(ctx, pipeEnd)` in a goroutine with a per-conversation cancelable context. The service holds the other pipe end and translates between file I/O and the QEMU framing `AcceptQemu` expects (4-byte big-endian length prefix + frame).
3. **Use**: the client opens `<n>/data` (and optionally `<n>/stats`, `<n>/type`) and does frame I/O.
4. **Reference counting**: every open handle on a file inside `<n>/` (including the one returned by `clone`) holds a reference. Opening a file in `<n>/` adds one; `Close` drops one. Path-only operations (`Stat`, `ReadDir`) do not hold references.
5. **Free**: when the reference count drops to zero, the context is canceled, the pipe is closed, and `<n>/` is removed. Ids are not reused for the lifetime of the service.

The usual sequence is: open `clone` → read id → open `<n>/data` → close the `clone` handle (the `data` handle keeps the conversation alive) → … → close `data`, which frees it.

**Session end before free**: if `AcceptQemu` returns (error or EOF), `hangup` is written, or the service is closed, the session ends but `<n>/` stays until its last handle is closed. After that point, pending and future `data` reads return `io.EOF`, `data` writes fail with `io.ErrClosedPipe`, and `stats` reports `closed`. Opening files in a closed conversation still works (so it can be inspected), but `data` I/O fails as above.

### Control file (`ctl`)

Newline-terminated text commands; surrounding whitespace ignored; unknown commands fail with a write error.

- **`connect <type>`**: optional, accepted for compatibility. Only `-1` is supported; any other value fails. Frames flow whether or not `connect` was written.
- **`hangup`**: end the session early (see “Session end before free”). It does not free the conversation; closing the handles does that.

### Data file (`data`)

- **Write**: the buffer is one complete Ethernet frame (dst MAC, src MAC, ethertype, payload; no FCS). It is sent to `vnet` with a length prefix. Writes shorter than 14 bytes or longer than `MTU + 14` fail with an error and nothing is sent.
- **Read**: blocks until `vnet` emits a frame for this NIC, then returns that single frame (length prefix stripped). If the read buffer is smaller than the frame, the frame is truncated and the remainder discarded (Plan 9 behavior).
- **Buffering**: frames from `vnet` are drained continuously into a bounded queue (256 frames) so a slow reader never stalls the switch; when the queue is full, new frames are dropped and counted in `stats`.
- **Concurrency**: one reader and one writer at a time are supported; frames are never interleaved within a single read or write. Reads should honor context cancellation where the runtime provides it.

The NIC's MAC address is whatever source MAC the guest uses; `vnet` learns it from traffic. Guests typically obtain an IP via DHCP from the gateway.

### Stats

Root `stats` (one `key: value` per line):

```
mtu: 1500
subnet: 192.168.127.0/24
gateway: 192.168.127.1
gatewaymac: 5a:94:ef:e4:0c:dd
conversations: 2
```

Per-conversation `stats`:

```
in: <frames read by client>
out: <frames written by client>
inbytes: <n>
outbytes: <n>
dropped: <frames dropped because the read queue was full>
state: open|closed[ (<error>)]
```

### Security / isolation

The service is a capability: mounting it grants raw L2 access to the virtual network (and through its NAT, to the host's network). Omit mounting to deny it.

### Test plan (minimum)

- Opening `clone` twice returns two `ctl` handles with distinct ids and creates `<n>/{ctl,data,type,stats}`; writing `connect -1` to that handle succeeds.
- Closing the `clone` handle while `<n>/data` is open keeps `<n>/`; closing the last handle removes it and cancels the `AcceptQemu` context.
- A DHCP DISCOVER written to `data` yields a DHCP OFFER on read.
- An ARP request for the gateway IP yields a reply with `gatewaymac`.
- `hangup` unblocks a pending `data` read with EOF and makes later writes fail, but `<n>/` stays until all handles are closed.
- Invalid `ctl` commands and `connect` with a type other than `-1` return errors.
- Undersized/oversized `data` writes fail without sending.
