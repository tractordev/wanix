package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	importsDir, err := filepath.Abs(os.Args[1])
	if err != nil {
		fatal(err.Error())
	}
	pkgStagingDir, err := filepath.Abs(os.Args[2])
	if err != nil {
		fatal(err.Error())
	}

	targets := []string{"js_wasm", "darwin_amd64"}
	for _, target := range targets {
		fmt.Printf("Building %s...\n", target)
		buildArchives(pkgStagingDir, importsDir, target)
		generateLinkConfig(pkgStagingDir, target)
	}

	// TODO: build compile.wasm and link.wasm

	// TODO: zip everything into build/pkg.zip
}

func buildArchives(workingDir, importsDir, target string) {
	if err := os.Chdir(importsDir); err != nil {
		fatal(err.Error())
	}

	GOOS, GOARCH, _ := strings.Cut(target, "_")
	cmd := exec.Command("go", "install", "-work", "-a", "-trimpath", "./...")
	cmd.Env = append(os.Environ(), "GOOS="+GOOS, "GOARCH="+GOARCH)

	fmt.Println(cmd.String())
	output, err := cmd.CombinedOutput()

	// Ignoring "imported but not used" errors.
	// It should have compiled the archives anyway.
	if err != nil && cmd.ProcessState.ExitCode() == 0 {
		if output != nil {
			fmt.Println(string(output))
		}

		fatal(err.Error())
	}

	rgx, err := regexp.Compile("WORK=(.*)")
	if err != nil {
		fatal(err.Error())
	}

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

	rgx, err = regexp.Compile("packagefile (.*)")
	if err != nil {
		fatal(err.Error())
	}

	outputDir := filepath.Join(workingDir, "targets", target)
	if err := os.MkdirAll(outputDir, 0655); err != nil {
		fatal(err.Error())
	}
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

		visited := map[string]struct{}{}
		unique := []string{}
		for _, m := range pkgMatches {
			str := string(m[1])
			if _, ok := visited[str]; !ok {
				visited[str] = struct{}{}
				unique = append(unique, str)
			}
		}

		for _, pkg := range unique {
			name, path, found := strings.Cut(pkg, "=")
			if !found {
				continue
			}

			newpath := filepath.Join(outputDir, name) + ".a"
			if err = os.MkdirAll(filepath.Dir(newpath), 0655); err != nil {
				fatal(err.Error())
			}
			if err = copyFile(path, newpath); err != nil {
				fatal(err.Error())
			}
		}
	}
}

func generateLinkConfig(workingDir, target string) {
	f, err := os.Create(filepath.Join(workingDir, "importcfg_"+target+".link"))
	if err != nil {
		fatal(err.Error())
	}
	defer f.Close()

	targetDir := filepath.Join(workingDir, "targets", target)
	err = fs.WalkDir(os.DirFS(targetDir), ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		// packagefile pathWithoutExt=/tmp/build/pkg/targets/target/path\n
		f.WriteString("packagefile " + path[:len(path)-2] + "=/tmp/build/pkg/targets/" + target + "/" + path + "\n")
		return nil
	})
	if err != nil {
		fatal(err.Error())
	}
}

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(dst, src)
	return err
}

func fatal(msg string) {
	fmt.Println(msg)
	os.Exit(1)
}
