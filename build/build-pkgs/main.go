package main

import (
	"archive/zip"
	"fmt"
	"go/build"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	importsDir, err := filepath.Abs(os.Args[1])
	if err != nil {
		fatal(err.Error())
	}
	pkgZipPath, err := filepath.Abs(os.Args[2])
	if err != nil {
		fatal(err.Error())
	}

	zf, err := os.Create(pkgZipPath)
	if err != nil {
		fatal(err.Error())
	}
	defer func() {
		if err := zf.Close(); err != nil {
			fatal(err.Error())
		}
	}()

	zw := zip.NewWriter(zf)
	defer func() {
		if err := zw.Close(); err != nil {
			fatal(err.Error())
		}
	}()

	targets := []string{"js_wasm", "darwin_amd64"}
	for _, target := range targets {
		fmt.Printf("Building target %s...\n", target)
		pkgNames := buildArchives(zw, importsDir, target)
		generateLinkConfig(zw, target, pkgNames)
	}

	buildTool(zw, "compile")
	buildTool(zw, "link")
}

func buildArchives(zw *zip.Writer, importsDir, target string) []string {
	if err := os.Chdir(importsDir); err != nil {
		fatal(err.Error())
	}

	GOOS, GOARCH, _ := strings.Cut(target, "_")
	cmd := exec.Command("go", "install", "-work", "-a", "-trimpath", "./...")
	cmd.Env = append(os.Environ(), "GOOS="+GOOS, "GOARCH="+GOARCH)

	// fmt.Println(cmd.String())
	output, err := cmd.CombinedOutput()

	// Ignoring "imported but not used" errors.
	// It should have compiled the archives anyway.
	if err != nil && cmd.ProcessState.ExitCode() == 0 {
		if output != nil {
			fmt.Println(string(output))
		}

		fatal(err.Error())
	}

	rgx := regexp.MustCompile("WORK=(.*)")
	rgxMatches := rgx.FindSubmatch(output)
	if rgxMatches == nil || rgxMatches[1] == nil {
		fatal("install output missing WORK path")
	}

	WORK := string(rgxMatches[1])
	// fmt.Println("WORK:", WORK)
	defer os.RemoveAll(WORK)

	workFS := os.DirFS(WORK)
	globMatches, err := fs.Glob(workFS, "*/importcfg")
	if err != nil {
		fatal(err.Error())
	}

	rgx = regexp.MustCompile("packagefile (.*)")

	visited := map[string]struct{}{}
	unique := []string{}
	for _, filename := range globMatches {
		contents, err := fs.ReadFile(workFS, filename)
		if err != nil {
			fmt.Printf("skipping %s: %s\n", filename, err)
			continue
		}

		pkgMatches := rgx.FindAllSubmatch(contents, -1)
		if pkgMatches == nil {
			// fmt.Printf("skipping %s: no packagefiles\n", filename)
			continue
		}

		for _, m := range pkgMatches {
			str := string(m[1])
			if _, ok := visited[str]; !ok {
				visited[str] = struct{}{}
				unique = append(unique, str)
			}
		}
	}

	targetDir := path.Join("pkg", "targets", target)
	for i, pkg := range unique {
		name, arPath, found := strings.Cut(pkg, "=")
		if !found {
			continue
		}

		if err = copyFileToZip(zw, path.Join(targetDir, name)+".a", arPath); err != nil {
			fatal(err.Error())
		}

		// overwrite with name so later we can pass unique to generateLinkConfig
		unique[i] = path.Clean(name)
	}

	return unique
}

func generateLinkConfig(zw *zip.Writer, target string, packages []string) {
	wr, err := zw.Create("pkg/importcfg_" + target + ".link")
	if err != nil {
		fatal(err.Error())
	}

	for _, pkg := range packages {
		io.WriteString(wr, "packagefile "+pkg+"=/tmp/build/pkg/targets/"+target+"/"+pkg+".a\n")
	}
}

func buildTool(zw *zip.Writer, name string) {
	// cd $GOROOT/src/cmd/$name && GOOS=js GOARCH=wasm go build -o $pkgDir/$name.wasm -trimpath
	fmt.Printf("Building %s.wasm...\n", name)

	if err := os.Chdir(path.Join(build.Default.GOROOT, "src", "cmd", name)); err != nil {
		fatal(err.Error())
	}
	tmpDir, err := os.MkdirTemp("", "wanix-build-*")
	if err != nil {
		fatal(err.Error())
	}
	defer os.RemoveAll(tmpDir)

	output := path.Join(tmpDir, name+".wasm")

	cmd := exec.Command("go", "build", "-o", output, "-trimpath")
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm", "GOTOOLCHAIN=go1.21.1")
	cmdOut, err := cmd.CombinedOutput()
	if err != nil {
		if cmdOut != nil {
			fmt.Println(string(cmdOut))
		}
		fatal(err.Error())
	}

	if err = copyFileToZip(zw, "pkg/"+name+".wasm", output); err != nil {
		fatal(err.Error())
	}
}

func copyFileToZip(zw *zip.Writer, dstPath, srcPath string) error {
	dst, err := zw.Create(dstPath)
	if err != nil {
		return err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	_, err = io.Copy(dst, src)
	return err
}

func fatal(msg string) {
	fmt.Println("FATAL:", msg)
	os.Exit(1)
}
