//go:build js && wasm

package wasi

import (
	"strings"

	"tractor.dev/wanix"
	wasiworker "tractor.dev/wanix/wasi/worker"
	"tractor.dev/wanix/web/worker"
)

type Driver struct {
	Workers *worker.Device
}

func (d *Driver) Check(t *wanix.Task) bool {
	// todo: wasi detection
	return strings.HasSuffix(t.Arg(0), ".wasm")
}

func (d *Driver) Start(t *wanix.Task) error {
	return worker.StartTaskWorker(d.Workers, t, wasiworker.BlobURL())
}
