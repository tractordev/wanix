//go:build js && wasm

package gojs

import (
	"tractor.dev/wanix"
	gojsworker "tractor.dev/wanix/gojs/worker"
	"tractor.dev/wanix/misc/wasmutil"
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
	if typ != "gojs" {
		return false
	}
	return true
}

func (d *Driver) Start(t *wanix.Task) error {
	return worker.StartTaskWorker(d.Workers, t, gojsworker.BlobURL())
}
