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

	"tractor.dev/toolkit-go/engine/fs/fsutil"
	"tractor.dev/wanix/kernel/proc/exec"
)

// returns an empty wasmPath on error or non-zero exit code
func buildCmdSource(path string) (wasmPath string, err error) {
	wasmPath = filepath.Join("/sys/bin", filepath.Base(path)+".wasm")

	// TODO: could also just change the search order in shell.go:findCommand()
	wasmExists, err := fsutil.Exists(os.DirFS("/"), unixToFsPath(wasmPath))
	if err != nil {
		return "", err
	}

	if !wasmExists {
		cmd := exec.Command("build", "-output", wasmPath, path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		exitCode, err := cmd.Run()
		if exitCode != 0 {
			return "", err
		}
	}

	return wasmPath, nil
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

func parsePrefixEnvArgs(args []string, env *[]string) (rest []string, err error) {
	for i, arg := range args {
		key, value, found := strings.Cut(arg, "=")
		if !found {
			rest = args[i:]
			break
		}
		if key == "" {
			return rest, fmt.Errorf("invalid variable at arg %d", i)
		}

		found = false
		for j, entry := range *env {
			if strings.HasPrefix(entry, key) {
				(*env)[j] = strings.Join([]string{key, value}, "=")
				found = true
				break
			}
		}
		if !found {
			*env = append(*env, strings.Join([]string{key, value}, "="))
		}
	}
	return rest, nil
}

// Parses $ENV_VAR arguments. `env` is a slice of "key=value" strings.
// Returns an error if it cannot find a matching environment
// variable.
func parseEnvArgs(args *[]string, env []string) error {
	// TODO: actually parse arguments
	for i, arg := range *args {
		envArg, found := strings.CutPrefix(arg, "$")
		if !found {
			continue
		}

		found = false
		for _, envVar := range env {
			if strings.HasPrefix(envVar, envArg) {
				(*args)[i] = envVar[len(envArg)+1:]
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("%w: cannot find environment variable %s", os.ErrInvalid, envArg)
		}
	}

	return nil
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
	r := make([]string, 0, len(m))
	for k, v := range m {
		r = append(r, strings.Join([]string{k, v}, "="))
	}
	return r
}

// Unix absolute path. Returns cwd if path is empty
func absPath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, path)
}

// Convert a Unix path to an io/fs path (See `io/fs.ValidPath()`)
// Use `absPath()` instead if passing result to OS functions
func unixToFsPath(path string) string {
	if !filepath.IsAbs(path) {
		// Join calls Clean internally
		wd, _ := os.Getwd()
		path = filepath.Join(strings.TrimLeft(wd, "/"), path)
	} else {
		path = filepath.Clean(strings.TrimLeft(path, "/"))
	}
	return path
}

func checkErr(w io.Writer, err error) (hadError bool) {
	if err != nil {
		io.WriteString(w, fmt.Sprintln(err))
		return true
	}
	return false
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
