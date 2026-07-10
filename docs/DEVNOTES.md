#### task exec protocol

Currently multiple codepoints for this but will eventually consolidate as the
protocol definition settles.

* rc/exec_wanix.go
* extras/wexec/main.go
* elements/task.js
* workbench/src/web/extension.ts

#### tinygo patch 

If you get this error building Tinygo:

```
% make wasm-tinygo
# net/http
/opt/homebrew/Cellar/tinygo/0.41.1/src/net/http/roundtrip_js.go:73:12: t.roundTrip undefined (type *Transport has no field or method roundTrip, but does have method RoundTrip)
make: *** [wasm-tinygo] Error 1
```

This is a known issue and the workaround until next release is a simple patch to
the file mentioned in the error:

```
# on linux
sed -i 's/t.roundTrip(req)/t.RoundTrip(req)/g' /path/to/net/http/roundtrip_js.go
# on mac
sed -i '' 's/t\.roundTrip(req)/t.RoundTrip(req)/g' /opt/homebrew/Cellar/tinygo/0.41.1/src/net/http/roundtrip_js.go
```