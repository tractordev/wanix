package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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

type BuildFlags struct {
	Os, Arch, Output *string
	PrintTargets     *bool
}

func setupCLI() *cli.Command {
	cmd := &cli.Command{
		Usage: "build [options] <packagePath>",
		Args:  cli.MinArgs(1),
		Short: "Compiles a go package.",
	}

	cFlags := cmd.Flags()
	inFlags := BuildFlags{
		Os:           cFlags.String("os", runtime.GOOS, "Sets the target OS (defaults to the host OS)"),
		Arch:         cFlags.String("arch", runtime.GOARCH, "Sets the target architecture (defaults to the host architecture)"),
		PrintTargets: cFlags.Bool("targets", false, "Print the supported build targets"),
		Output:       cFlags.String("output", "", "Outputs the executable to the given filepath or directory (defaults to cwd)"),
	}

	cmd.Run = func(ctx *cli.Context, args []string) {
		os.Exit(main2(inFlags, args))
	}

	return cmd
}

// TODO: experiment with including the stdlib source instead of our hacky link archives.

func main() {
	if err := cli.Execute(context.Background(), setupCLI(), os.Args[1:]); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

//go:embed pkg.zip
var zipEmbed []byte

func main2(flags BuildFlags, args []string) int {
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

	absPkgPath, err := filepath.Abs(args[0])
	if err != nil {
		fmt.Println(err)
		return 1
	}

	pkgs, err := parser.ParseDir(token.NewFileSet(), absPkgPath, nil, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		if pkgs == nil {
			fmt.Printf("error reading package directory '%s': %s", absPkgPath, err)
		} else {
			fmt.Printf("parse error: %s", err)
		}
		return 1
	}

	// TODO:
	// Should we compile directories containing multiple packages?
	// You'd have to analyze the import-graph to figure out which subpackages to include though.
	if len(pkgs) != 1 {
		fmt.Println("Build only supports building a single package, found: %d", len(pkgs))
		return 1
	}

	var pkg *ast.Package
	for _, pkg = range pkgs {
	}

	// using maps to ensure deduplication
	embedPatterns := make(map[string]struct{})
	imports := make(map[string]struct{})
	filePaths := make([]string, 0, len(pkg.Files))

	for fpath, file := range pkg.Files {
		patterns := findEmbedDirectives(file.Comments)
		for _, p := range patterns {
			if _, ok := embedPatterns[p]; !ok {
				embedPatterns[p] = struct{}{}
			}
		}

		for _, i := range file.Imports {
			if _, ok := imports[i.Path.Value]; !ok {
				imports[i.Path.Value] = struct{}{}
			}
		}

		// fpath should be absolute since we passed an absolute path to ParseDir
		filePaths = append(filePaths, fpath)
	}

	if err := os.MkdirAll("/sys/tmp/build", 0755); err != nil {
		fmt.Println(err)
		return 1
	}

	// Generate embedcfg
	hasEmbeds := len(embedPatterns) > 0
	var embedcfgPath string = ""
	if hasEmbeds {
		embedcfgPath, err = generateEmbedConfig(embedPatterns, absPkgPath)
		if err != nil {
			fmt.Println("unable to generate embedcfg:", err)
			return 1
		}
	}

	// Generate importcfg
	importcfgPath := strings.Join([]string{"/sys/tmp/build/importcfg", target.os, target.arch}, "_")
	importcfg, err := os.Create(importcfgPath)
	if err != nil {
		fmt.Println("unable to create importcfg:", err)
		return 1
	}

	bw := bufio.NewWriter(importcfg)
	for i := range imports {
		fmt.Fprintf(bw, "packagefile %s=/sys/build/pkg/targets/%s_%s/%[1]s.a\n", strings.Trim(i, "\""), target.os, target.arch)
	}
	if err := bw.Flush(); err != nil {
		fmt.Println("unable to write to importcfg:", err)
		return 1
	}
	if err := importcfg.Close(); err != nil {
		fmt.Println("unable to write to importcfg:", err)
		return 1
	}

	if err := openZipPkg("/sys/build", target); err != nil {
		fmt.Println("unable to open pkg.zip:", err)
		return 1
	}

	objPath := fmt.Sprintf("/sys/tmp/build/%s.a", pkg.Name)

	compileArgs := []string{
		"-p=main",
		"-complete",
		"-dwarf=false",
		"-pack",
		"-o", objPath,
		"-importcfg", importcfgPath,
	}
	if hasEmbeds {
		compileArgs = append(compileArgs, "-embedcfg", embedcfgPath)
	}
	compileArgs = append(compileArgs, filePaths...)

	// run compile.wasm
	fmt.Printf("Compiling %s to %s\n", args[0], objPath)
	exitcode, err := run("/sys/build/pkg/compile.wasm", compileArgs...)
	if exitcode != 0 || err != nil {
		if err != nil {
			fmt.Println(err)
			if exitcode == 0 {
				exitcode = 1
			}
		}
		return exitcode
	}

	linkcfg := fmt.Sprintf("/sys/build/pkg/importcfg_%s_%s.link", target.os, target.arch)

	output, err := getOutputPath(target.arch, *flags.Output, pkg.Name, absPkgPath)
	if err != nil {
		fmt.Println(err)
		return 1
	}

	// run link.wasm using importcfg_$GOOS_$GOARCH.link
	fmt.Println("Linking", objPath)
	exitcode, err = run(
		"/sys/build/pkg/link.wasm",
		"-importcfg", linkcfg,
		"-buildmode=exe",
		"-o", output,
		objPath,
	)
	if exitcode != 0 || err != nil {
		if err != nil {
			fmt.Println(err)
			if exitcode == 0 {
				exitcode = 1
			}
		}
		return exitcode
	}

	fmt.Println("Output", output)
	return 0
}

func findEmbedDirectives(comments []*ast.CommentGroup) (patterns []string) {
	for _, group := range comments {
		for _, cmnt := range group.List {
			pattern, found := strings.CutPrefix(cmnt.Text, "//go:embed ")
			if !found {
				continue
			}

			patterns = append(patterns, pattern)
		}
	}
	return
}

func getOutputPath(arch, outputArg, pkgName, pkgDir string) (string, error) {
	var output string

	// var moduleName string
	// if pkg.Name == "main" && absPkgPath != "/" {
	// 	moduleName = filepath.Base(absPkgPath)
	// } else {
	// 	moduleName = pkg.Name
	// }

	var moduleName string
	if pkgDir == "." || pkgDir == "/" {
		moduleName = pkgName
	} else {
		moduleName = filepath.Base(pkgDir)
	}

	if arch == "wasm" {
		moduleName += ".wasm"
	}

	if outputArg != "" {
		outputArg = filepath.Clean(outputArg)

		if isdir, err := fsutil.DirExists(os.DirFS("/"), strings.TrimLeft(outputArg, "/")); isdir {
			output = filepath.Join(outputArg, moduleName)
		} else if err != nil {
			return output, err
		} else {
			output = outputArg
		}
	} else {
		wd, err := os.Getwd()
		if err != nil {
			return output, err
		}

		output = filepath.Join(wd, moduleName)
	}

	return output, nil
}

func run(name string, args ...string) (int, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func openZipPkg(dest string, tgt Target) error {
	fmt.Printf("Unpacking pkg.zip/%s to %s...\n", tgt.print(), dest)

	pkg, err := zip.NewReader(bytes.NewReader(zipEmbed), int64(len(zipEmbed)))
	if err != nil {
		return err
	}

	os.MkdirAll(filepath.Join(dest, "pkg"), 0755)

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

func generateEmbedConfig(patterns map[string]struct{}, srcDir string) (cfgPath string, err error) {
	jsonObj := struct {
		Patterns map[string][]string
		Files    map[string]string
	}{
		make(map[string][]string, len(patterns)), make(map[string]string),
	}

	dfs := os.DirFS(srcDir)
	for pattern := range patterns {
		matches, err := fs.Glob(dfs, pattern)
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
						jsonObj.Patterns[pattern] = append(jsonObj.Patterns[pattern], path)
						jsonObj.Files[path], _ = filepath.Abs(filepath.Join(srcDir, path))
					}

					return nil
				})
				if err != nil {
					return "", err
				}
			} else {
				jsonObj.Patterns[pattern] = append(jsonObj.Patterns[pattern], m)
				jsonObj.Files[m], _ = filepath.Abs(filepath.Join(srcDir, m))
			}

			f.Close()
		}
	}

	cfgPath = "/sys/tmp/build/embedcfg"
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
