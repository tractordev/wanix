//go:build js && wasm

package wasi

import (
	"tractor.dev/wanix"
	"tractor.dev/wanix/misc/wasmutil"
	wasiworker "tractor.dev/wanix/wasi/worker"
	"tractor.dev/wanix/web/worker"
)

type Driver struct {
	Workers *worker.Device
}

func (d *Driver) Check(t *wanix.Task) bool {
	typ, err := wasmutil.DetectType(t.NS(), t.Arg(0))
	if err != nil {
		// log.Println("error detecting wasm type", err)
		return false
	}
	if typ != "wasi" {
		return false
	}
	return true
}

func (d *Driver) Start(t *wanix.Task) error {
	return worker.StartTaskWorker(d.Workers, t, wasiworker.BlobURL())
}
