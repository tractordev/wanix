package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall/js"
	"time"
	"unsafe"

	"github.com/anmitsu/go-shlex"
	"github.com/spf13/afero"
	"golang.org/x/term"
	"tractor.dev/toolkit-go/engine/fs"
)

// todo: these should all be able to be removed in place of os.Getwd, os.Getenv
var shellEnv = map[string]string{}
var exeEnv = map[string]string{}

type CmdType int

const (
	CmdIsScript CmdType = iota
	CmdIsSourceDir
	CmdIsWasm
)

type CmdSearchResult struct {
	CmdType
	found bool
	path  string
}

// Search environment and/or input path to find an executable
func searchForCommand(path string) (result CmdSearchResult) {
	dfs := os.DirFS("/")

	if result = getCmdByPath(dfs, path); result.found {
		return
	}

	// start searching cmd directories
	// TODO: search for cmd directories instead of just these predefined ones.
	// TODO: search shell env PATH for executable.

	exeName := filepath.Base(path)
	if result = getCmdByPath(dfs, filepath.Join("/cmd", exeName)); result.found {
		return
	}
	if result = getCmdByPath(dfs, filepath.Join("/sys/cmd", exeName)); result.found {
		return
	}
	if result = getCmdByPath(dfs, filepath.Join("/sys/bin", exeName)); result.found {
		return
	}

	return CmdSearchResult{found: false}
}

// Checks for file at `path`, `path.wasm`, `path.sh`, and a directory at `path`.
func getCmdByPath(iofs fs.FS, path string) CmdSearchResult {
	path = absPath(path)

	isfile, err := fileExists(iofs, unixToFsPath(path))
	if err == nil && isfile {
		var ct CmdType
		if strings.HasSuffix(path, ".sh") {
			ct = CmdIsScript
		} else {
			ct = CmdIsWasm
		}

		return CmdSearchResult{
			found:   true,
			path:    path,
			CmdType: ct,
		}
	}

	wasm_path := strings.Join([]string{path, ".wasm"}, "")
	isfile, err = fileExists(iofs, unixToFsPath(wasm_path))
	if err == nil && isfile {
		return CmdSearchResult{
			found:   true,
			path:    wasm_path,
			CmdType: CmdIsWasm,
		}
	}

	script_path := strings.Join([]string{path, ".sh"}, "")
	isfile, err = fileExists(iofs, unixToFsPath(script_path))
	if err == nil && isfile {
		return CmdSearchResult{
			found:   true,
			path:    script_path,
			CmdType: CmdIsScript,
		}
	}

	if isDir, _, err := dirExists(iofs, unixToFsPath(path)); err == nil && isDir {
		return CmdSearchResult{
			found:   true,
			path:    path,
			CmdType: CmdIsSourceDir,
		}
	}

	return CmdSearchResult{found: false}
}

func buildCmdSource(path string) (exePath string, err error) {
	// TODO: Implement
	fmt.Println("TODO: Implement building commands from source")
	return filepath.Join("/sys/bin", filepath.Base(path)+".wasm"), nil
}

type ExitCode struct {
	code int
	err  error
}

func (e ExitCode) check(w io.Writer) bool {
	if e.code != 0 {
		if e.err != nil {
			io.WriteString(w, fmt.Sprintln(e.err))
		}
	}
	return e.code != 0
}

var WASM_MAGIC = []byte{0, 'a', 's', 'm'}

func runWasm(rpc io.Reader, t *term.Terminal, afs afero.Fs, env map[string]string, path string, args []string) ExitCode {
	fsPath := unixToFsPath(path)
	data, err := afero.ReadFile(afs, fsPath)
	if err != nil {
		return ExitCode{1, err}
	}

	if !bytes.HasPrefix(data, WASM_MAGIC) {
		return ExitCode{1, errors.New(fmt.Sprintf("cant exec %s: non WASM file", absPath(path)))}
	}

	buf := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(buf, data)

	var stdout = js.FuncOf(func(this js.Value, args []js.Value) any {
		buf := make([]byte, args[0].Length())
		js.CopyBytesToGo(buf, args[0])
		go t.Write(buf)
		return nil
	})
	defer stdout.Release()

	// more micro hardcoding:
	// resolve the path if given file to open,
	// otherwise it can't seem to find files in
	// the wd for some reason
	if fsPath == "cmd/micro.wasm" && len(args) > 0 {
		args[0] = unixToFsPath(args[0])
	}

	// TODO: these unpack functions are dumb. Why don't types coerce to any?
	// wanix.exec(wasm, args, env, stdout, stderr)
	promise := js.Global().Get("wanix").Call("exec", buf, unpackArray(args), unpackMap(env), stdout, stdout)

	// wait for process to finish
	wait := make(chan *ExitCode)
	var exit *ExitCode
	if fsPath == "cmd/micro.wasm" {
		// pipe rpc input to stdin global
		// it's all smoke 'n' mirrors baby
		ctx, cancel := context.WithCancel(context.Background())
		promise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
			cancel()
			go func() {
				// send a byte over js side of rpc channel to cancel the copy
				<-time.After(500 * time.Millisecond)
				buf := js.Global().Get("Uint8Array").New(1)
				js.CopyBytesToJS(buf, []byte{' '})
				js.Global().Get("wanix").Get("termCh").Call("write", buf)
			}()
			return nil
		}))

		cr := cancelableReader{
			r:   rpc,
			ctx: ctx,
		}
		// Copy() blocks when reading from rpc, only stopping when cancel() is called.
		// This lets us block and pipe stdin until the process is finished

		_, err = io.Copy(io.Discard, cr)
		if err != nil && err != context.Canceled {
			panic(err) // TODO: maybe remove this?
			exit = &ExitCode{1, err}
		} else {
			exit = &ExitCode{0, nil}
		}
	} else {
		then := js.FuncOf(func(this js.Value, args []js.Value) any {
			wait <- &ExitCode{args[0].Int(), nil}
			return nil
		})
		defer then.Release()
		promise.Call("then", then)
	}
	if exit == nil {
		exit = <-wait
	}
	fmt.Println("Program exited", exit.code)
	return *exit
}

func runScript(t *term.Terminal, path string, args []string) {
	fmt.Println("TODO: run shell script in child process")
	// exec("shell", path, args, t)
}

func parseEnvVars(t *term.Terminal, args []string) (ok bool, overrideEnv map[string]string, argsRest []string) {
	for i, arg := range args {
		name, value, found := strings.Cut(arg, "=")
		if !found {
			argsRest = args[i:]
			break
		}
		if name == "" {
			io.WriteString(t, fmt.Sprintf("invalid environment variable (%d): missing variable name\n", i))
			return false, overrideEnv, args
		}
		// delay making the override until we absolutely need it. Ensures it's null if empty
		if overrideEnv == nil {
			overrideEnv = make(map[string]string)
		}
		overrideEnv[name] = value
	}

	return true, overrideEnv, argsRest
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

type cancelableReader struct {
	r   io.Reader
	ctx context.Context
}

func (cr cancelableReader) Read(p []byte) (n int, err error) {
	select {
	case <-cr.ctx.Done():
		return 0, cr.ctx.Err()
	default:
		n, err := cr.r.Read(p)
		return n, err
	}
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

// DirExists checks if a path exists and is a directory.
// copied from afero/util, edited to return FileInfo
func dirExists(iofs fs.FS, path string) (bool, os.FileInfo, error) {
	fi, err := fs.Stat(iofs, path)
	if err == nil && fi.IsDir() {
		return true, fi, nil
	}
	if os.IsNotExist(err) {
		return false, fi, nil
	}
	return false, fi, err
}

// fileExists checks if a path exists and is a file.
func fileExists(iofs fs.FS, path string) (bool, error) {
	fi, err := fs.Stat(iofs, path)
	// TODO: maybe just check !fi.IsDir()?
	if err == nil && fi.Mode().IsRegular() {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// TEMP: just for fsdata debug command
func GetUnexportedField(field reflect.Value) interface{} {
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface()
}
