module tractor.dev/wanix/v86

go 1.26.0

replace github.com/hugelgupf/p9 => github.com/progrium/p9 v0.0.0-20251108235831-1c1dfeb38c1e

replace golang.org/x/sys => github.com/progrium/sys-wasm v0.0.0-20240620081741-5ccc4fc17421

replace tractor.dev/wanix => ../

require (
	github.com/evanw/esbuild v0.28.0
	tractor.dev/wanix v0.0.0-20260430024630-620f178de142
)

require golang.org/x/sys v0.45.0 // indirect
