module tractor.dev/wanix/rc

go 1.25.0

replace golang.org/x/sys => github.com/progrium/sys-wasm v0.0.0-20240620081741-5ccc4fc17421

require (
	github.com/dustin/go-humanize v1.0.1
	github.com/u-root/u-root v0.16.0
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701
	mvdan.cc/sh/v3 v3.13.1
)

require (
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/term v0.41.0 // indirect
)
