//go:build js && wasm

package gojs

import (
	"strings"

	"tractor.dev/wanix"
	gojsworker "tractor.dev/wanix/gojs/worker"
	"tractor.dev/wanix/web/worker"
)

// todo: move to gojs dir?
type Driver struct {
	Workers *worker.Device
}

func (d *Driver) Check(t *wanix.Task) bool {
	// todo: gojs detection
	return strings.HasSuffix(t.Arg(0), ".wasm")
}

func (d *Driver) Start(t *wanix.Task) error {
	return worker.StartTaskWorker(d.Workers, t, gojsworker.BlobURL())
}
