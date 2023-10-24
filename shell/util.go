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
	"tractor.dev/wanix/kernel/fs"
)

// todo: these should all be able to be removed in place of os.Getwd, os.Getenv
var shellEnv = map[string]string{}
var exeEnv = map[string]string{}

// Search environment and/or input path to find an executable
func findExecutable(t *term.Terminal, fs afero.Fs, path string, firstAttempt bool) (exePath string, found, isScript bool) {
	isfile, err := fileExists(fs, unixToFsPath(path))
	if err == nil && isfile {
		return path, true, false
	}

	wasm_path := strings.Join([]string{unixToFsPath(path), ".wasm"}, "")
	isfile, err = fileExists(fs, wasm_path)
	if err == nil && isfile {
		return "/" + wasm_path, true, false
	}

	script_path := strings.Join([]string{unixToFsPath(path), ".sh"}, "")
	isfile, err = fileExists(fs, script_path)
	if err == nil && isfile {
		return script_path, true, true
	}

	// TODO: search shell env PATH for executable
	if firstAttempt {
		return findExecutable(t, fs, filepath.Join("/cmd", filepath.Base(path)), false)
	} else {
		return "", false, false
	}
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

		_, err = io.Copy(fs.StdinBuf, cr)
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

func runScript(rpcChannel io.Reader, t *term.Terminal, fs afero.Fs, path string, args []string) {
	data, err := afero.ReadFile(fs, unixToFsPath(path))
	if checkErr(t, err) {
		return
	}

	data = bytes.TrimSpace(data)
	lines := strings.Split(string(data), "\n")

	for i, line := range lines {
		// adapted from (api *terminalAPI) Open() above
		// TODO: make an API for executing commands. This is only copied
		// to do custom error handling.
		if len(line) == 0 {
			continue
		}

		lArgs, err := shlex.Split(line, true)
		if err != nil {
			io.WriteString(t, fmt.Sprintf("error on line %d: parsing error\n", i))
			return
		}

		// Setup child process environment
		ok, overrideEnv, lArgs := parseEnvVars(t, lArgs)
		if !ok {
			continue
		}
		if len(lArgs) == 0 {
			io.WriteString(t, fmt.Sprintf("error on line %d: missing command or executable\n", i))
			continue
		}
		if overrideEnv == nil {
			exeEnv = shellEnv
		} else {
			exeEnv = make(map[string]string, len(shellEnv)+len(overrideEnv))
			for k, v := range shellEnv {
				exeEnv[k] = v
			}
			for k, v := range overrideEnv {
				exeEnv[k] = v
			}
		}

		for i, a := range lArgs {
			if strings.Contains(a, "$1") {
				lArgs[i] = strings.Replace(a, "$1", args[0], -1)
			}
		}

		// io.WriteString(t, line+"\n")

		if cmd, ok := commands[lArgs[0]]; ok {
			cmd(t, fs, lArgs[1:])
		} else if exe, found, isScript := findExecutable(t, fs, lArgs[0], true); found {
			if isScript {
				runScript(rpcChannel, t, fs, exe, lArgs[1:])
			} else {
				exit := runWasm(rpcChannel, t, fs, exeEnv, exe, lArgs[1:])
				if exit.check(t) {
					io.WriteString(t, fmt.Sprintf("script error on line %d\n", i))
					return
				}
			}
		} else {
			io.WriteString(t, fmt.Sprintf("script error on line %d: command or executable not found\n", i))
			return
		}
	}

	io.WriteString(t, "\n")
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
	if filepath.IsAbs(path) || path == "" {
		return filepath.Clean(path)
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, path)
}

// Convert a Unix path to an io/fs path (See `io/fs.ValidPath()`)
func unixToFsPath(path string) string {
	if !filepath.IsAbs(path) {
		// Join calls Clean internally
		wd, _ := os.Getwd()
		path = filepath.Join(wd, path)
	} else {
		path = filepath.Clean(path)
	}
	return path //strings.TrimLeft(path, "/") <-- with os calls we dont do this
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
func dirExists(fs afero.Fs, path string) (bool, os.FileInfo, error) {
	fi, err := fs.Stat(path)
	if err == nil && fi.IsDir() {
		return true, fi, nil
	}
	if os.IsNotExist(err) {
		return false, fi, nil
	}
	return false, fi, err
}

// fileExists checks if a path exists and is a file.
func fileExists(fs afero.Fs, path string) (bool, error) {
	fi, err := fs.Stat(path)
	// maybe just check !fi.IsDir()?
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
