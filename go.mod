module tractor.dev/wanix

go 1.25

replace github.com/hugelgupf/p9 => github.com/progrium/p9 v0.0.0-20250227010111-4025760ecd04

replace golang.org/x/sys => github.com/progrium/sys-wasm v0.0.0-20240620081741-5ccc4fc17421

// patch on top of the feature/cbor-tinygo-beta branch
// based on https://github.com/fxamacker/cbor/issues/686
replace github.com/fxamacker/cbor/v2 => ./hack/cbor

require (
	github.com/fxamacker/cbor/v2 v2.9.0
	github.com/gorilla/websocket v1.5.3
	github.com/hanwen/go-fuse/v2 v2.7.2
	github.com/hugelgupf/p9 v0.3.1-0.20240118043522-6f4f11e5296e
	github.com/progrium/go-netstack v0.0.0-20240720002214-37b2b8227b91
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701
	golang.org/x/net v0.39.0
	golang.org/x/sys v0.32.0
	golang.org/x/term v0.31.0
	tractor.dev/toolkit-go v0.0.0-20250103001615-9a6753936c19
)

require (
	github.com/apparentlymart/go-cidr v1.1.0 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/inetaf/tcpproxy v0.0.0-20240214030015-3ce58045626c // indirect
	github.com/miekg/dns v1.1.58 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/mod v0.18.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.22.0 // indirect
)
