package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/toolkit-go/engine/fs/fsutil"
	"tractor.dev/wanix/kernel/proc/exec"
)

type Target struct {
	os   string
	arch string
}

func (t Target) print() string {
	return strings.Join([]string{t.os, t.arch}, "_")
}

func (target Target) isValid() bool {
	for _, t := range supportedTargets {
		if target == t {
			return true
		}
	}
	return false
}

var supportedTargets = []Target{
	{"js", "wasm"}, {"darwin", "amd64"},
}

type SourceInfo struct {
	dirPath  string
	filename string
}

func (si SourceInfo) filePath() string {
	return filepath.Join(si.dirPath, si.filename)
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
			return SourceInfo{}, fmt.Errorf("%w: missing main.go in directory %s", os.ErrNotExist, path)
		} else {
			return SourceInfo{dirPath: filepath.Clean(path), filename: "main.go"}, nil
		}
	} else {
		return SourceInfo{dirPath: filepath.Dir(path), filename: filepath.Base(path)}, nil
	}
}

type BuildFlags struct {
	Os, Arch, Output *string
	PrintTargets     *bool
}

func setupCLI() *cli.Command {
	cmd := &cli.Command{
		Usage: "build [options] <srcPath>",
		Args:  cli.MinArgs(1),
		Short: "Compiles a go file or source directory.",
	}

	cFlags := cmd.Flags()
	inFlags := BuildFlags{
		Os:           cFlags.String("os", runtime.GOOS, "Sets the target OS (defaults to the host OS)"),
		Arch:         cFlags.String("arch", runtime.GOARCH, "Sets the target architecture (defaults to the host architecture)"),
		PrintTargets: cFlags.Bool("targets", false, "Print the supported build targets"),
		Output:       cFlags.String("output", "", "Outputs an executable to the given filepath or directory (defaults to cwd)"),
	}

	cmd.Run = func(ctx *cli.Context, args []string) {
		os.Exit(mainWithExitCode(inFlags, args))
	}

	return cmd
}

func main() {
	if err := cli.Execute(context.Background(), setupCLI(), os.Args[1:]); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

//go:embed pkg.zip
var zipEmbed []byte

func mainWithExitCode(flags BuildFlags, args []string) int {
	if *flags.PrintTargets {
		fmt.Println("Supported targets:")
		for _, t := range supportedTargets {
			fmt.Printf("OS=%s ARCH=%s\n", t.os, t.arch)
		}
		return 0
	}

	target := Target{os: *flags.Os, arch: *flags.Arch}

	if !target.isValid() {
		fmt.Printf("unsupported build target OS=%s ARCH=%s\n", target.os, target.arch)
		fmt.Println("use -targets flag to see supported targets")
		return 1
	}

	srcInfo, err := getSourceInfo(args[0])
	if err != nil {
		fmt.Println(err)
		return 1
	}

	src, err := os.ReadFile(srcInfo.filePath())
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

	// TODO: use os Temp funcs instead
	if err := os.MkdirAll("/tmp/build", 0755); err != nil {
		fmt.Println(err)
		return 1
	}
	// TEMPORARY: only commented out to make testing easier.
	// Ensure we clean up all build artifacts from here on
	// defer func() {
	// 	if err := os.RemoveAll("/tmp/build"); err != nil {
	// 		fmt.Println("unable to clean build artifacts:", err)
	// 	}
	// }()

	// TODO: unpack in a different tmp folder, so successive builds don't
	// require unpacking everything each time.
	fmt.Println("Unpacking pkg.zip...")
	if err := openZipPkg("/tmp/build", target); err != nil {
		fmt.Println("unable to open pkg.zip:", err)
		return 1
	}

	embedPatterns, hasEmbeds := findEmbedDirectives(src)
	var embedcfgPath string = ""
	if hasEmbeds {
		embedcfgPath, err = generateEmbedConfig(embedPatterns, srcInfo.dirPath)
		if err != nil {
			fmt.Println("unable to generate embedcfg:", err)
			return 1
		}
	}

	importcfgPath := strings.Join([]string{"/tmp/build/importcfg", target.os, target.arch}, "_")
	importcfg, err := os.Create(importcfgPath)
	if err != nil {
		fmt.Println("unable to create importcfg:", err)
		return 1
	}

	bw := bufio.NewWriter(importcfg)
	for _, i := range ast.Imports {
		fmt.Fprintf(bw, "packagefile %s=/tmp/build/pkg/targets/%s_%s/%[1]s.a\n", strings.Trim(i.Path.Value, "\""), target.os, target.arch)
	}
	if err := bw.Flush(); err != nil {
		fmt.Println("unable to write to importcfg:", err)
		return 1
	}
	importcfg.Close()

	objPath := fmt.Sprintf("/tmp/build/%s.a", strings.TrimSuffix(srcInfo.filename, ".go"))

	compileArgs := []string{
		"-p=main",
		"-complete",
		"-dwarf=false",
		"-pack",
		"-o", objPath,
		"-importcfg", importcfgPath,
		// "-I", "/tmp/build/pkg/targets/js_wasm/", // TODO: I think this can replace our generated importcfg
		"-v",
		srcInfo.filePath(),
	}
	if hasEmbeds {
		compileArgs = append([]string{"-embedcfg", embedcfgPath}, compileArgs...)
	}

	fmt.Println("Compiling", args[0])
	// run compile.wasm
	if exitcode, err := run("/tmp/build/pkg/compile.wasm", compileArgs...); exitcode != 0 {
		if err != nil {
			fmt.Println(err)
		}
		return exitcode
	}

	linkcfg := fmt.Sprintf("/tmp/build/pkg/importcfg_%s_%s.link", target.os, target.arch)

	// TODO: make sure it outputs a ".wasm" file in the output dir
	var output string
	if *flags.Output != "" {
		output = *flags.Output
	} else if srcInfo.dirPath == "." || srcInfo.dirPath == "/" {
		output, _, _ = strings.Cut(srcInfo.filename, ".")
	} else {
		output = filepath.Base(srcInfo.dirPath)
	}

	fmt.Println("Linking", objPath, output)
	// run link.wasm using importcfg_$GOOS_$GOARCH.link
	if exitcode, err := run(
		"/tmp/build/pkg/link.wasm",
		"-importcfg", linkcfg, // TODO: maybe we can use importcfgPath instead?
		"-buildmode=exe",
		"-o", output,
		objPath,
	); exitcode != 0 {
		if err != nil {
			fmt.Println(err)
		}
		return exitcode
	}

	return 0
}

func run(name string, args ...string) (int, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func openZipPkg(dest string, tgt Target) error {
	pkg, err := zip.NewReader(bytes.NewReader(zipEmbed), int64(len(zipEmbed)))
	if err != nil {
		return err
	}

	dfs := os.DirFS(dest)

	// TODO: Optimize
	for _, zfile := range pkg.File {
		if strings.HasPrefix(zfile.Name, "pkg/targets") {
			if !strings.HasPrefix(zfile.Name, "pkg/targets/"+tgt.print()) {
				continue
			}
		}

		if exists, err := fsutil.Exists(dfs, filepath.Clean(zfile.Name)); exists {
			continue
		} else if err != nil {
			return err
		}

		if zfile.FileInfo().IsDir() {
			if err := os.MkdirAll(filepath.Join(dest, zfile.Name), 0755); err != nil {
				return err
			}
		} else {
			zreader, err := zfile.Open()
			defer zreader.Close()
			if err != nil {
				return err
			}

			destFile, err := os.Create(filepath.Join(dest, zfile.Name))
			defer destFile.Close()
			if err != nil {
				return err
			}

			if _, err := io.Copy(destFile, zreader); err != nil {
				return err
			}
		}
	}
	return nil
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

func generateEmbedConfig(patterns []string, srcDir string) (cfgPath string, err error) {
	jsonObj := struct {
		Patterns map[string][]string
		Files    map[string]string
	}{
		make(map[string][]string, len(patterns)), make(map[string]string),
	}

	dfs := os.DirFS(srcDir)
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
				err := fs.WalkDir(dfs, m, func(path string, d fs.DirEntry, walkErr error) error {
					if walkErr != nil {
						return walkErr
					}

					if !d.IsDir() {
						jsonObj.Patterns[p] = append(jsonObj.Patterns[p], path)
						jsonObj.Files[path], _ = filepath.Abs(filepath.Join(srcDir, path))
					}

					return nil
				})
				if err != nil {
					return "", err
				}
			} else {
				jsonObj.Patterns[p] = append(jsonObj.Patterns[p], m)
				jsonObj.Files[m], _ = filepath.Abs(filepath.Join(srcDir, m))
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
