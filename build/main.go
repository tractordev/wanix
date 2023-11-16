package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall/js"
)

func Usage() string {
	return `Compiles a go file.
Usage: build <-targets|source file>

Outputs the compiled binary next to the input source file, 
using the name of the containing directory if possible.

GOOS and GOARCH match the host values by default,
but can be overridden by environment variables.

-targets flag prints the supported build targets.`
}

type Target struct {
	os   string
	arch string
}

var supportedTargets = []Target{
	{"js", "wasm"}, {"darwin", "amd64"},
}

func getBuildTarget() (target Target, valid bool) {
	target.os = os.Getenv("GOOS")
	if target.os == "" {
		target.os = runtime.GOOS
	}

	target.arch = os.Getenv("GOARCH")
	if target.arch == "" {
		target.arch = runtime.GOARCH
	}

	valid = false
	for _, t := range supportedTargets {
		if target == t {
			valid = true
			break
		}
	}

	return target, valid
}

type SourceInfo struct {
	dir_path  string
	file_name string
}

func (si SourceInfo) filePath() string {
	return filepath.Join(si.dir_path, si.file_name)
}

func getSourceInfo(path string) (SourceInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return SourceInfo{}, err
	}
	defer f.Close()

	fstat, err := f.Stat()
	if err != nil {
		return SourceInfo{}, err
	}

	// if path is a directory, search dir for main.go
	// if path is a go file, get parent dir
	// TODO: user created directories show up as files for some reason.
	if fstat.IsDir() {
		entries, err := f.Readdirnames(-1)
		if err != nil {
			return SourceInfo{}, err
		}

		found := false
		for _, e := range entries {
			if e == "main.go" {
				found = true
				break
			}
		}

		if !found {
			return SourceInfo{}, errors.New("input directory missing a main.go")
		} else {
			return SourceInfo{dir_path: filepath.Clean(path), file_name: "main.go"}, nil
		}
	} else {
		return SourceInfo{dir_path: filepath.Dir(path), file_name: filepath.Base(path)}, nil
	}
}

func main() {
	os.Exit(mainWithExitCode())
}

func mainWithExitCode() int {
	args := os.Args[1:]

	if len(args) != 1 {
		fmt.Println(Usage())
		return 1
	}

	if args[0] == "-targets" {
		fmt.Println("Supported targets:")
		for _, t := range supportedTargets {
			fmt.Printf("GOOS=%s GOARCH=%s\n", t.os, t.arch)
		}
		return 0
	}

	// verify valid GOOS and GOARCH targets
	target, valid := getBuildTarget()
	if !valid {
		fmt.Printf("unsupported build target GOOS=%s GOARCH=%s\n", target.os, target.arch)
		fmt.Println("use -targets flag to see supported targets")
		return 1
	}

	src_info, err := getSourceInfo(args[0])
	if err != nil {
		fmt.Println(err)
		return 1
	}

	src, err := os.ReadFile(src_info.filePath())
	if err != nil {
		fmt.Println("unable to read source file:", err)
		return 1
	}

	// parse src imports and generate an importcfg file
	fset := token.NewFileSet()
	ast, err := parser.ParseFile(fset, "", src, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		fmt.Println("unable to parse source file:", err)
		return 1
	}

	embedPatterns, hasEmbeds := findEmbedDirectives(src)
	var embedcfgPath string = ""
	if hasEmbeds {
		embedcfgPath, err = generateEmbedConfig(embedPatterns, src_info.dir_path)
		if err != nil {
			fmt.Println("unable to generate embedcfg:", err)
			return 1
		}
		defer os.Remove(embedcfgPath) // defer applies to function scope!
	}

	importcfgPath := strings.Join([]string{"/tmp/build/importcfg", target.os, target.arch}, "_")

	importcfg, err := os.Create(importcfgPath)
	if err != nil {
		fmt.Println("unable to create importcfg:", err)
		return 1
	}
	defer func() {
		if err := os.Remove(importcfgPath); err != nil {
			fmt.Println("unable to remove importcfg:", err)
		}
	}()

	bw := bufio.NewWriter(importcfg)
	for _, i := range ast.Imports {
		fmt.Fprintf(bw, "packagefile %s=/tmp/build/pkg/%s_%s/%[1]s.a\n", strings.Trim(i.Path.Value, "\""), target.os, target.arch)
	}
	if err := bw.Flush(); err != nil {
		fmt.Println("unable to write to importcfg:", err)
		return 1
	}
	importcfg.Close()

	objPath := fmt.Sprintf("/tmp/build/%s.a", strings.TrimSuffix(src_info.file_name, ".go"))

	compileArgs := []string{
		"-p", "main",
		"-complete",
		"-dwarf=false",
		"-pack",
		"-o", objPath,
		"-importcfg", importcfgPath,
		src_info.filePath(),
	}
	if hasEmbeds {
		compileArgs = append([]string{"-embedcfg", embedcfgPath}, compileArgs...)
	}
	env := mapEnv()

	fmt.Println("Compiling", args[0])
	// run compile.wasm
	exitcode := runWasm("/cmd/compile.wasm", compileArgs, env)
	if exitcode.code != 0 {
		if exitcode.err != nil {
			fmt.Println(exitcode.err)
		}
		return exitcode.code
	}

	defer func() {
		if err := os.Remove(objPath); err != nil {
			fmt.Println("unable to remove build artifact:", err)
		}
	}()

	fmt.Println("Linking", objPath)
	// run link.wasm using importcfg_$GOOS_$GOARCH.link
	linkcfg := fmt.Sprintf("/tmp/build/pkg/importcfg_%s_%s.link", target.os, target.arch)
	exitcode = runWasm("/cmd/link.wasm", []string{"-importcfg", linkcfg, "-buildmode=exe", objPath}, env)
	if exitcode.code != 0 {
		if exitcode.err != nil {
			fmt.Println(exitcode.err)
		}
		return exitcode.code
	}

	var output_name string
	if src_info.dir_path == "." || src_info.dir_path == "/" {
		output_name, _, _ = strings.Cut(src_info.file_name, ".")
	} else {
		output_name = filepath.Base(src_info.dir_path)
	}
	// TODO: link.wasm dumps a.out in the cwd instead of src_info.dir_path
	os.Rename("a.out", filepath.Join(src_info.dir_path, output_name))

	// profit
	return 0
}

func findEmbedDirectives(src []byte) (patterns []string, ok bool) {
	r := regexp.MustCompile(`//go:embed (.*)`)
	matches := r.FindAllSubmatch(src, -1)

	if matches == nil {
		return nil, false
	}

	// append patterns without duplicates
	allKeys := make(map[string]bool)
	for _, m := range matches {
		p := strings.TrimSpace(string(m[1]))
		if _, value := allKeys[p]; !value {
			allKeys[p] = true
			patterns = append(patterns, string(m[1]))
		}
	}

	return patterns, true
}

func generateEmbedConfig(patterns []string, src_dir string) (cfgPath string, err error) {
	jsonObj := struct {
		Patterns map[string][]string
		Files    map[string]string
	}{
		make(map[string][]string, len(patterns)), make(map[string]string),
	}

	dfs := os.DirFS(src_dir)
	for _, p := range patterns {
		matches, err := fs.Glob(dfs, patterns[0])
		if err != nil {
			return "", err
		}

		for _, m := range matches {
			// I'm assuming since fs.Glob returned this without error,
			// it's safe to open these files.
			f, _ := dfs.Open(m)
			stat, _ := f.Stat()

			if stat.IsDir() {
				err := fs.WalkDir(dfs, m, func(path string, d fs.DirEntry, walk_err error) error {
					if walk_err != nil {
						return walk_err
					}

					if !d.IsDir() {
						jsonObj.Patterns[p] = append(jsonObj.Patterns[p], path)
						jsonObj.Files[path], _ = filepath.Abs(filepath.Join(src_dir, path))
					}

					return nil
				})
				if err != nil {
					return "", err
				}
			} else {
				jsonObj.Patterns[p] = append(jsonObj.Patterns[p], m)
				jsonObj.Files[m], _ = filepath.Abs(filepath.Join(src_dir, m))
			}

			f.Close()
		}
	}

	cfgPath = "/tmp/build/embedcfg"
	cfg, err := os.Create(cfgPath)
	if err != nil {
		return cfgPath, err
	}

	j := json.NewEncoder(cfg)
	if err = j.Encode(jsonObj); err != nil {
		return cfgPath, err
	}

	cfg.Close()

	return cfgPath, nil
}

type ExitCode struct {
	code int
	err  error
}

func runWasm(path string, args []string, env map[string]any) ExitCode {
	wasm, err := os.ReadFile(path)
	if err != nil {
		return ExitCode{1, err}
	}

	buf := js.Global().Get("Uint8Array").New(len(wasm))
	js.CopyBytesToJS(buf, wasm)

	var stdout = js.FuncOf(func(this js.Value, args []js.Value) any {
		buf := make([]byte, args[0].Length())
		js.CopyBytesToGo(buf, args[0])
		os.Stdout.Write(buf)
		return nil
	})
	defer stdout.Release()

	// wanix.exec(wasm, args, env, stdout, stderr)
	promise := js.Global().Get("wanix").Call("exec", buf, unpackArray(args), env, stdout, stdout)

	wait := make(chan ExitCode)
	then := js.FuncOf(func(this js.Value, args []js.Value) any {
		wait <- ExitCode{args[0].Int(), nil}
		return nil
	})
	defer then.Release()

	// TODO: not sure this is necessary
	catch := js.FuncOf(func(this js.Value, args []js.Value) any {
		wait <- ExitCode{1, errors.New(args[0].Get("message").String())}
		return nil
	})
	defer catch.Release()

	promise.Call("then", then).Call("catch", catch)
	return <-wait
}

func mapEnv() map[string]any {
	env := os.Environ()
	var result = make(map[string]any, len(env))

	for _, envVar := range env {
		k, v, _ := strings.Cut(envVar, "=")
		result[k] = v
	}

	return result
}

func unpackArray[S ~[]E, E any](s S) []any {
	r := make([]any, len(s))
	for i, e := range s {
		r[i] = e
	}
	return r
}
