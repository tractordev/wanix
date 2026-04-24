//go:build js && wasm

package web

import (
	"strings"
	"syscall/js"

	"tractor.dev/wanix"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/pipe"
	gojsworker "tractor.dev/wanix/gojs/worker"
	"tractor.dev/wanix/jsutil"
	wasiworker "tractor.dev/wanix/wasi/worker"
	"tractor.dev/wanix/web/caches"
	"tractor.dev/wanix/web/dl"
	"tractor.dev/wanix/web/dom"
	"tractor.dev/wanix/web/fsa"
	"tractor.dev/wanix/web/worker"
)

func startWorkerTask(svc *worker.Service, t *wanix.Task, blobURL string) error {
	w, err := svc.Alloc(t)
	if err != nil {
		return err
	}
	args := append([]string{blobURL}, strings.Split(t.Cmd(), " ")...)
	return w.Start(args...)
}

type gojsDriver struct {
	svc *worker.Service
}

func (d *gojsDriver) Check(t *wanix.Task) bool {
	// todo: gojs detection
	return strings.HasSuffix(t.Arg(0), ".wasm")
}

func (d *gojsDriver) Start(t *wanix.Task) error {
	return startWorkerTask(d.svc, t, gojsworker.BlobURL())
}

type wasiDriver struct {
	svc *worker.Service
}

func (d *wasiDriver) Check(t *wanix.Task) bool {
	// todo: wasi detection
	return strings.HasSuffix(t.Arg(0), ".wasm")
}

func (d *wasiDriver) Start(t *wanix.Task) error {
	return startWorkerTask(d.svc, t, wasiworker.BlobURL())
}

type jsDriver struct {
	svc  *worker.Service
	root *wanix.Task
}

func (d *jsDriver) Check(t *wanix.Task) bool {
	return strings.HasSuffix(t.Arg(0), ".js")
}

func (d *jsDriver) Start(t *wanix.Task) error {
	data, err := fs.ReadFile(d.root.Namespace(), t.Arg(0))
	if err != nil {
		return err
	}
	jsBuf := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(jsBuf, data)
	blob := js.Global().Get("Blob").New([]any{jsBuf}, js.ValueOf(map[string]any{"type": "text/javascript"}))
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	return startWorkerTask(d.svc, t, url.String())
}

func New(root *wanix.Task) fskit.MapFS {
	workerfs := worker.New(root)
	opfs, _ := fsa.OPFS()
	webfs := fskit.MapFS{
		"console": jsutil.ConsoleFS,
		"dom":     dom.New(),
		"caches":  caches.New(),
		"worker":  workerfs,
		"opfs":    opfs,
		"dl":      dl.New(),
	}
	// if !runtime.Instance().Get("_sw").IsUndefined() {
	// 	webfs["sw"] = sw.Activate(runtime.Instance().Get("_sw"), k)
	// 	webfs["sw"] = sw.Activate(runtime.Instance().Get("_sw"), k)
	// }

	root.Register("js", &jsDriver{svc: workerfs, root: root})
	root.Register("wasi", &wasiDriver{svc: workerfs})
	root.Register("gojs", &gojsDriver{svc: workerfs})
	return webfs
}

type dataFile struct {
	js.Value
	buf *pipe.Buffer
}

func (s *dataFile) Stat() (fs.FileInfo, error) {
	return fskit.Entry("data", 0644), nil
}

func (s *dataFile) Write(p []byte) (n int, err error) {
	buf := js.Global().Get("Uint8Array").New(len(p))
	n = js.CopyBytesToJS(buf, p)
	s.Value.Call("send", buf)
	return
}

func (s *dataFile) Read(p []byte) (int, error) {
	return s.buf.Read(p)
}

func (s *dataFile) Close() error {
	return nil
}
