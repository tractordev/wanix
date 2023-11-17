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
	"unsafe"

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
	return "", errors.New("TODO: Implement building commands from source")
	// TODO: Implement
	// return filepath.Join("/sys/bin", filepath.Base(path)+".wasm"), nil
}

var WASM_MAGIC = []byte{0, 'a', 's', 'm'}

func isWasmFile(name string) (bool, error) {
	if f, err := os.Open(name); err != nil {
		return false, err
	} else {
		magic := make([]byte, 4)
		if _, err := f.Read(magic); err != nil {
			return false, err
		}

		return bytes.Equal(magic, WASM_MAGIC), nil
	}
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

func unpackMap2(m map[string]string) []string {
	r := make([]string, len(m))
	for k, v := range m {
		r = append(r, strings.Join([]string{k, v}, "="))
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
