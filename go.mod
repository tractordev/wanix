module tractor.dev/wanix

go 1.21.1

require (
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be
	github.com/evanw/esbuild v0.19.5
	github.com/spf13/afero v1.10.0
	golang.org/x/term v0.13.0
	tractor.dev/toolkit-go v0.0.0-20231020134529-7767e3b09e40
)

require (
	github.com/fxamacker/cbor/v2 v2.5.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
)

replace tractor.dev/toolkit-go => ../toolkit-go
