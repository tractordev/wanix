module tractor.dev/wanix/v86

go 1.25.0

replace github.com/hugelgupf/p9 => github.com/progrium/p9 v0.0.0-20251108235831-1c1dfeb38c1e

replace golang.org/x/sys => github.com/progrium/sys-wasm v0.0.0-20240620081741-5ccc4fc17421

replace tractor.dev/wanix => ../

require (
	github.com/hugelgupf/p9 v0.3.1-0.20240118043522-6f4f11e5296e
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701
	tractor.dev/toolkit-go v0.0.0-20250103001615-9a6753936c19
	tractor.dev/wanix v0.0.0-20260430024630-620f178de142
)

require (
	github.com/evanw/esbuild v0.28.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
)
