module tractor.dev/wanix

go 1.23.1

replace github.com/hugelgupf/p9 => github.com/progrium/p9 v0.0.0-20250227004919-318e829e843d

require (
	github.com/hanwen/go-fuse/v2 v2.7.2
	github.com/hugelgupf/p9 v0.3.1-0.20240118043522-6f4f11e5296e
	github.com/magefile/mage v1.15.0
	tractor.dev/toolkit-go v0.0.0-20250103001615-9a6753936c19
)

require (
	github.com/fxamacker/cbor/v2 v2.5.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
)
