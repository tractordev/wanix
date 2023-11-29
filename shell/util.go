package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"unsafe"

	"tractor.dev/wanix/kernel/proc/exec"
)

func buildCmdSource(path string) (wasmPath string, err error) {
	cmd := exec.Command("build", "-output", "/sys/bin/", path)
	_, err = cmd.Run()
	return filepath.Join("/sys/bin", filepath.Base(path)+".wasm"), err
}

var WASM_MAGIC = []byte{0, 'a', 's', 'm'}

func isWasmFile(name string) bool {
	f, err := os.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	magic := make([]byte, 4)
	if _, err := f.Read(magic); err != nil {
		return false
	}
	return bytes.Equal(magic, WASM_MAGIC)
}

func parseEnvArgs(args []string, env map[string]string) (rest []string, err error) {
	for i, arg := range args {
		name, value, found := strings.Cut(arg, "=")
		if !found {
			rest = args[i:]
			break
		}
		if name == "" {
			return rest, fmt.Errorf("invalid variable at arg %d", i)
		}
		env[name] = value
	}
	return rest, nil
}

func unpackArray[S ~[]E, E any](s S) []any {
	r := make([]any, len(s))
	for i, e := range s {
		r[i] = e
	}
	return r
}

func unpackMap(m map[string]string) map[string]any {
	r := make(map[string]any, len(m))
	for k, v := range m {
		r[k] = v
	}
	return r
}

func packEnv(m map[string]string) []string {
	r := make([]string, len(m))
	for k, v := range m {
		r = append(r, strings.Join([]string{k, v}, "="))
	}
	return r
}

// TEMP: just for fsdata debug command
func GetUnexportedField(field reflect.Value) interface{} {
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface()
}

type SwitchableWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (sw *SwitchableWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.writer.Write(p)
}

func (sw *SwitchableWriter) Switch(w io.Writer) {
	sw.mu.Lock()
	sw.writer = w
	sw.mu.Unlock()
}

// BlockingBuffer is an io.ReadWriter that blocks on read when empty.
type BlockingBuffer struct {
	buf    bytes.Buffer
	mu     sync.Mutex
	cond   *sync.Cond
	closed bool
}

// NewBlockingBuffer creates a new BlockingBuffer.
func NewBlockingBuffer() *BlockingBuffer {
	bb := &BlockingBuffer{}
	bb.cond = sync.NewCond(&bb.mu)
	return bb
}

// Write writes data to the buffer and wakes up any blocked readers.
func (bb *BlockingBuffer) Write(p []byte) (n int, err error) {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	if bb.closed {
		return 0, io.ErrClosedPipe
	}

	n, err = bb.buf.Write(p)
	bb.cond.Broadcast() // Wake up blocked readers
	return n, err
}

// Read reads data from the buffer. It blocks if the buffer is empty.
func (bb *BlockingBuffer) Read(p []byte) (n int, err error) {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	for bb.buf.Len() == 0 && !bb.closed {
		bb.cond.Wait() // Wait for data to be written
	}

	if bb.closed {
		return 0, io.EOF
	}

	return bb.buf.Read(p)
}

// Close marks the buffer as closed and wakes up any blocked readers.
func (bb *BlockingBuffer) Close() error {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	bb.closed = true
	bb.cond.Broadcast() // Wake up blocked readers
	return nil
}
