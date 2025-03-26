//go:build js && wasm

package jsutil

import (
	"io"
	"strings"
	"syscall/js"
)

func Log(args ...any) {
	js.Global().Get("console").Call("log", args...)
}

func Get(path string) (v js.Value) {
	parts := strings.Split(path, ".")
	v = js.Global()
	for _, part := range parts {
		v = v.Get(part)
	}
	return
}

func AsyncIter(iter js.Value, fn func(js.Value) error) (err error) {
	var res js.Value
	res, err = AwaitErr(iter.Call("next"))
	if err != nil {
		return
	}
	for !res.Get("done").Bool() {
		if err = fn(res.Get("value")); err != nil {
			return
		}
		res, err = AwaitErr(iter.Call("next"))
		if err != nil {
			return
		}
	}
	return nil
}

type Writer struct {
	js.Value
}

func (w *Writer) Write(p []byte) (n int, err error) {
	buf := js.Global().Get("Uint8Array").New(len(p))
	js.CopyBytesToJS(buf, p)
	nn, e := AwaitErr(w.Value.Call("write", buf))
	if e != nil {
		return 0, e
	}
	n = nn.Int()
	return
}

func (w *Writer) Close() error {
	w.Value.Call("close")
	return nil
}

type Reader struct {
	js.Value
}

func (r *Reader) Read(p []byte) (n int, err error) {
	buf := js.Global().Get("Uint8Array").New(len(p))
	nn, e := AwaitErr(r.Value.Call("read", buf))
	if e != nil {
		return 0, e
	}
	js.CopyBytesToGo(p, buf)
	if nn.IsNull() {
		return 0, io.EOF
	}
	return nn.Int(), nil
}

func (r *Reader) Close() error {
	r.Value.Call("close")
	return nil
}

type ReadWriter struct {
	io.ReadCloser
	io.WriteCloser
}

func (rw *ReadWriter) Close() (err error) {
	err = rw.ReadCloser.Close()
	if err != nil {
		return
	}
	err = rw.WriteCloser.Close()
	return
}
