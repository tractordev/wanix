//go:build js && wasm

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"sync"
	"syscall/js"

	"tractor.dev/wanix/fs/pipe"
)

// P9Channel bridges a message-oriented RPC API to the stream-oriented
// io.ReadCloser / io.WriteCloser pair a 9P server expects.
//
// Pass Reader() and Writer() to the 9P server. Call Handle() for each
// incoming request; the matching response is delivered to `send`.
type P9Channel struct {
	reqR *io.PipeReader
	reqW *io.PipeWriter
	ch   js.Value

	mu      sync.Mutex
	pending map[uint16]func([]byte) // tag -> response callback
	respBuf []byte                  // accumulator for partial responses
}

func NewP9Channel() *P9Channel {
	ch := js.Global().Get("MessageChannel").New()
	r, w := io.Pipe()
	h := &P9Channel{
		ch:      ch,
		reqR:    r,
		reqW:    w,
		pending: make(map[uint16]func([]byte)),
	}
	ch.Get("port1").Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
		jsBuf := args[0].Get("data")
		buf := make([]byte, jsBuf.Get("byteLength").Int())
		js.CopyBytesToGo(buf, jsBuf)
		err := h.handle(buf, func(resp []byte) {
			jsBuf := js.Global().Get("Uint8Array").New(len(resp))
			js.CopyBytesToJS(jsBuf, resp)
			h.ch.Get("port1").Call("postMessage", jsBuf)
		})
		if err != nil {
			log.Println("9p handle error:", err)
		}
		return nil
	}))
	return h
}

// Reader returns the stream the 9P server should read requests from.
func (h *P9Channel) Reader() io.ReadCloser { return h.reqR }

// Writer returns the stream the 9P server should write responses to.
func (h *P9Channel) Writer() io.WriteCloser { return &respWriter{h: h} }

func (h *P9Channel) Port() js.Value { return h.ch.Get("port2") }

// Handle accepts one 9P request and registers the callback that will
// receive the response with the matching tag. Safe for concurrent use.
func (h *P9Channel) handle(req []byte, send func(resp []byte)) error {
	if len(req) < 7 {
		return fmt.Errorf("9P request too short: %d bytes", len(req))
	}
	tag := binary.LittleEndian.Uint16(req[5:7])

	h.mu.Lock()
	h.pending[tag] = send
	h.mu.Unlock()

	_, err := h.reqW.Write(req)
	return err
}

// Close tears down the bridge; the 9P server will see EOF on its reader.
func (h *P9Channel) Close() error { return h.reqW.Close() }

type respWriter struct{ h *P9Channel }

func (w *respWriter) Write(p []byte) (int, error) {
	w.h.mu.Lock()
	w.h.respBuf = append(w.h.respBuf, p...)

	type ready struct {
		send func([]byte)
		msg  []byte
	}
	var dispatched []ready

	for {
		if len(w.h.respBuf) < 7 {
			break // need at least size[4] + type[1] + tag[2]
		}
		size := int(binary.LittleEndian.Uint32(w.h.respBuf[:4]))
		if size < 7 {
			w.h.mu.Unlock()
			return len(p), fmt.Errorf("invalid 9P message size: %d", size)
		}
		if len(w.h.respBuf) < size {
			break // full message hasn't arrived yet
		}

		msg := make([]byte, size)
		copy(msg, w.h.respBuf[:size])
		w.h.respBuf = w.h.respBuf[size:]

		tag := binary.LittleEndian.Uint16(msg[5:7])
		if send, ok := w.h.pending[tag]; ok {
			delete(w.h.pending, tag)
			dispatched = append(dispatched, ready{send, msg})
		}
		// Unknown tag (e.g. response after Tflush) is silently dropped.
	}
	w.h.mu.Unlock()

	// Dispatch outside the lock so callbacks can't deadlock us.
	for _, r := range dispatched {
		r.send(r.msg)
	}
	return len(p), nil
}

func (w *respWriter) Close() error { return nil }

// P9PortReadWriter wraps a JavaScript MessagePort as an io.ReadWriteCloser for
// 9P-over-postMessage framing: each outgoing postMessage is one complete 9P
// packet (matching P9Channel on the WASM side). Stream-oriented Writes are
// buffered until the size header indicates a full message. Incoming messages
// are concatenated into a byte stream for Read (one postMessage per peer frame).
type P9PortReadWriter struct {
	port   js.Value
	rbuf   *pipe.Buffer
	onRecv js.Func
	mu     sync.Mutex
	wbuf   []byte
}

func NewP9PortReadWriter(port js.Value) *P9PortReadWriter {
	rbuf := pipe.NewBuffer(true)
	onRecv := js.FuncOf(func(this js.Value, args []js.Value) any {
		data := js.Global().Get("Uint8Array").New(args[0].Get("data"))
		buf := make([]byte, data.Length())
		js.CopyBytesToGo(buf, data)
		_, err := rbuf.Write(buf)
		if err != nil {
			log.Println("p9 port readwriter: rbuf.Write:", err)
		}
		return nil
	})
	port.Set("onmessage", onRecv)
	return &P9PortReadWriter{
		port:   port,
		rbuf:   rbuf,
		onRecv: onRecv,
	}
}

func (p *P9PortReadWriter) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	p.wbuf = append(p.wbuf, b...)

	nOut := len(b)
	for {
		if len(p.wbuf) < 7 {
			break
		}
		size := int(binary.LittleEndian.Uint32(p.wbuf[:4]))
		if size < 7 {
			return nOut, fmt.Errorf("invalid 9P message size: %d", size)
		}
		if len(p.wbuf) < size {
			break
		}
		jsBuf := js.Global().Get("Uint8Array").New(size)
		js.CopyBytesToJS(jsBuf, p.wbuf[:size])
		p.port.Call("postMessage", jsBuf)
		p.wbuf = append(p.wbuf[:0], p.wbuf[size:]...)
	}
	return nOut, nil
}

func (p *P9PortReadWriter) Read(pb []byte) (int, error) {
	return p.rbuf.Read(pb)
}

func (p *P9PortReadWriter) Close() error {
	_ = p.rbuf.Close()
	p.onRecv.Release()
	p.port.Set("onmessage", js.Undefined())
	return nil
}
