//go:build js && wasm

package jsutil

import (
	"io"
	"log"
	"strings"
	"syscall/js"

	"tractor.dev/wanix/fs/pipe"
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

func ToSlice(v js.Value) []js.Value {
	length := v.Length()
	slice := make([]js.Value, length)
	for i := 0; i < length; i++ {
		slice[i] = v.Index(i)
	}
	return slice
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
	buf js.Value // optional reusable buffer
}

// NewWriter creates a writer with an optional reusable buffer for better performance.
// Pass bufSize > 0 to enable buffer reuse, or 0 to allocate on each write.
func NewWriter(v js.Value, bufSize int) *Writer {
	w := &Writer{Value: v}
	if bufSize > 0 {
		w.buf = js.Global().Get("Uint8Array").New(bufSize)
	}
	return w
}

func (w *Writer) Write(p []byte) (n int, err error) {
	var buf js.Value
	if w.buf.IsUndefined() || w.buf.Length() < len(p) {
		buf = js.Global().Get("Uint8Array").New(len(p))
	} else {
		// Create a subarray view limited to len(p) to avoid writing stale buffer data
		buf = w.buf.Call("subarray", 0, len(p))
	}

	js.CopyBytesToJS(buf, p)
	nn, e := AwaitErr(w.Value.Call("write", buf))
	if e != nil {
		return 0, e
	}
	n = nn.Int()

	// Handle short writes
	// if n < len(p) {
	// 	return n, io.ErrShortWrite
	// }

	return n, nil
}

func (w *Writer) Close() error {
	w.Value.Call("close")
	return nil
}

// wraps a Go-style reader in JS to io.ReadCloser
// NOT for use with ReadableStream
type Reader struct {
	js.Value
	buf js.Value // optional reusable buffer
}

// NewReader creates a reader with an optional reusable buffer for better performance.
// Pass bufSize > 0 to enable buffer reuse, or 0 to allocate on each read.
func NewReader(v js.Value, bufSize int) *Reader {
	r := &Reader{Value: v}
	if bufSize > 0 {
		r.buf = js.Global().Get("Uint8Array").New(bufSize)
	}
	return r
}

func (r *Reader) Read(p []byte) (n int, err error) {
	var buf js.Value
	if r.buf.IsUndefined() || r.buf.Length() < len(p) {
		buf = js.Global().Get("Uint8Array").New(len(p))
	} else {
		// Create a subarray view limited to len(p) to avoid reading more than p can hold
		buf = r.buf.Call("subarray", 0, len(p))
	}

	nn, e := AwaitErr(r.Value.Call("read", buf))
	if e != nil {
		return 0, e
	}

	// Check EOF first before copying
	if nn.IsNull() {
		return 0, io.EOF
	}

	// Only copy the bytes that were actually read
	n = nn.Int()
	if n > 0 {
		js.CopyBytesToGo(p[:n], buf)
	}

	return n, nil
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

// wraps a MessagePort as an io.ReadWriteCloser
type PortReadWriter struct {
	port js.Value
	rbuf *pipe.Buffer
	wbuf js.Value
}

func NewPortReadWriter(port js.Value) *PortReadWriter {
	rbuf := pipe.NewBuffer(true)
	port.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
		data := js.Global().Get("Uint8Array").New(args[0].Get("data"))
		buf := make([]byte, data.Length())
		js.CopyBytesToGo(buf, data)
		_, err := rbuf.Write(buf)
		if err != nil {
			log.Println("error writing to rbuf:", err)
		}
		return nil
	}))
	return &PortReadWriter{
		port: port,
		rbuf: rbuf,
		wbuf: js.Global().Get("Uint8Array").New(8192),
	}
}

func (prw *PortReadWriter) Write(p []byte) (n int, err error) {
	var buf js.Value
	if prw.wbuf.IsUndefined() || prw.wbuf.Length() < len(p) {
		buf = js.Global().Get("Uint8Array").New(len(p))
	} else {
		// Create a subarray view limited to len(p) to avoid writing stale buffer data
		buf = prw.wbuf.Call("subarray", 0, len(p))
	}
	n = js.CopyBytesToJS(buf, p)
	prw.port.Call("postMessage", buf) // no transfer, we reuse our buffer
	return
}

func (prw *PortReadWriter) Read(p []byte) (int, error) {
	return prw.rbuf.Read(p)
}

func (prw *PortReadWriter) Close() error {
	return nil
}

// wraps a ReadableStream as an io.ReadCloser
type ReadableStream struct {
	reader   js.Value
	closed   bool
	leftover []byte
}

func NewReadableStream(stream js.Value) *ReadableStream {
	reader := stream.Call("getReader")
	return &ReadableStream{
		reader:   reader,
		closed:   false,
		leftover: nil,
	}
}

func (rsr *ReadableStream) Read(p []byte) (int, error) {
	if rsr.closed {
		return 0, io.EOF
	}

	// If there is leftover from a previous read, consume that first
	if len(rsr.leftover) > 0 {
		n := copy(p, rsr.leftover)
		rsr.leftover = rsr.leftover[n:]
		if len(rsr.leftover) == 0 {
			rsr.leftover = nil
		}
		return n, nil
	}

	// Read from the readable stream
	promise := rsr.reader.Call("read")
	result, err := AwaitErr(promise)
	if err != nil {
		return 0, err
	}
	if result.IsUndefined() || !result.Truthy() {
		return 0, io.EOF
	}

	if result.Get("done").Bool() {
		rsr.closed = true
		return 0, io.EOF
	}

	value := result.Get("value")
	// value is a Uint8Array
	if value.IsNull() || value.IsUndefined() {
		return 0, io.EOF
	}
	length := value.Get("length").Int()
	if length == 0 {
		return 0, nil
	}
	buf := make([]byte, length)
	js.CopyBytesToGo(buf, value)
	n := copy(p, buf)
	// If there is more data than we can return, save the leftover for next read
	if n < len(buf) {
		rsr.leftover = buf[n:]
	}
	return n, nil
}

func (rsr *ReadableStream) Close() error {
	if !rsr.closed {
		rsr.reader.Call("releaseLock")
		rsr.closed = true
	}
	return nil
}
