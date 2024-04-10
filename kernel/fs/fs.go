package fs

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/toolkit-go/engine/fs/memfs"
	"tractor.dev/toolkit-go/engine/fs/watchfs"

	"tractor.dev/wanix/internal"
	"tractor.dev/wanix/internal/githubfs"
	"tractor.dev/wanix/internal/httpfs"
	"tractor.dev/wanix/internal/indexedfs"
	"tractor.dev/wanix/internal/jsutil"
	"tractor.dev/wanix/internal/mountablefs"
	"tractor.dev/wanix/internal/procfs"
	"tractor.dev/wanix/kernel/proc"
)

var DebugLog string
var doLogging bool = DebugLog == "true"

func log(args ...any) {
	if doLogging {
		js.Global().Get("console").Call("log", args...)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type Service struct {
	fsys fs.MutableFS
	// Wraps fsys, so it's actually the same filesystem.
	watcher *watchfs.FS

	fds    map[int]*fd
	nextFd int

	mu sync.Mutex
}

type fd struct {
	fs.File
	Path string
}

func (s *Service) FS() fs.FS {
	return s.fsys
}

func (s *Service) Initialize(kernelSource embed.FS, p *proc.Service) {
	s.fds = make(map[int]*fd)
	s.nextFd = 1000

	ifs, err := indexedfs.New()
	if err != nil {
		panic(err)
	}
	s.fsys = mountablefs.New(ifs)
	s.watcher = watchfs.New(s.fsys)

	// ensure basic system tree exists
	fs.MkdirAll(s.fsys, "app", 0755)
	fs.MkdirAll(s.fsys, "cmd", 0755)
	fs.MkdirAll(s.fsys, "sys/app", 0755)
	fs.MkdirAll(s.fsys, "sys/bin", 0755)
	fs.MkdirAll(s.fsys, "sys/cmd", 0755)
	fs.MkdirAll(s.fsys, "sys/dev", 0755)
	fs.MkdirAll(s.fsys, "sys/tmp", 0755)
	fs.MkdirAll(s.fsys, "sys/build", 0755)

	devURL := fmt.Sprintf("%ssys/dev", js.Global().Get("hostURL").String())
	devMode := false
	resp, err := http.DefaultClient.Get(devURL)
	must(err)
	if resp.StatusCode == 200 {
		devMode = true
		must(s.fsys.(*mountablefs.FS).Mount(httpfs.New(devURL), "/sys/dev"))
	}

	// copy some apps including terminal
	must(copyAllFS(s.fsys, "sys/app/terminal", internal.Dir, "app/terminal"))
	if !devMode {
		must(copyAllFS(s.fsys, "sys/app/todo", internal.Dir, "app/todo"))
		must(copyAllFS(s.fsys, "sys/app/jazz-todo", internal.Dir, "app/jazz-todo"))
		must(copyAllFS(s.fsys, "sys/app/explorer", internal.Dir, "app/explorer"))
	}

	must(copyAllFS(s.fsys, "sys/cmd/kernel", kernelSource, "."))

	// Copy initfs files
	fs.MkdirAll(s.fsys, "sys/cmd/shell", 0755)
	shellFiles := getPrefixedInitFiles("shell/")
	for _, path := range shellFiles {
		must(s.copyFromInitFS(filepath.Join("sys/cmd", path), path))
	}

	if exists, _ := fs.Exists(s.fsys, "sys/cmd/build.wasm"); !exists {
		must(s.copyFromInitFS("sys/cmd/build.wasm", "bin/build"))
	}
	if exists, _ := fs.Exists(s.fsys, "sys/cmd/micro.wasm"); !exists {
		must(s.copyFromInitFS("sys/cmd/micro.wasm", "bin/micro"))
	}
	if exists, _ := fs.Exists(s.fsys, "sys/bin/shell.wasm"); !exists {
		must(s.copyFromInitFS("sys/bin/shell.wasm", "bin/shell"))
	}

	// setup exportapp
	fs.MkdirAll(s.fsys, "sys/export", 0755)
	must(s.copyFromInitFS("sys/export/main.go", "export/main.go"))
	must(s.copyFromInitFS("sys/cmd/exportapp.sh", "export/exportapp.sh"))

	// Mount custom filesystems
	must(s.fsys.(*mountablefs.FS).Mount(memfs.New(), "/sys/tmp"))

	s.maybeMountGithubFS()

	fs.MkdirAll(s.fsys, "sys/proc", 0755)
	must(s.fsys.(*mountablefs.FS).Mount(
		procfs.New(p),
		"/sys/proc",
	))
}

// Mount githubfs if user has gh_token
func (s *Service) maybeMountGithubFS() {
	u, err := jsutil.WanixSyscall("host.currentUser")
	if err != nil || u.IsNull() {
		return
	}

	m := u.Get("user_metadata")
	if m.IsUndefined() {
		return
	}

	token := m.Get("gh_token")
	if token.IsUndefined() {
		return
	}

	fs.MkdirAll(s.fsys, "repo", 0755)
	must(s.fsys.(*mountablefs.FS).Mount(
		githubfs.New(
			"wanixdev",
			"wanix.sh",
			token.String(),
		),
		"/repo",
	))
}

func getPrefixedInitFiles(prefix string) []string {
	names := js.Global().Get("Object").Call("getOwnPropertyNames", js.Global().Get("initfs"))
	length := names.Length()

	var result []string
	for i := 0; i < length; i += 1 {
		name := names.Index(i).String()
		if strings.HasPrefix(name, prefix) {
			result = append(result, name)
		}
	}

	return result
}

func copyFileAcrossFS(dstFS fs.MutableFS, dstPath string, srcFS fs.FS, srcPath string) error {
	srcData, err := fs.ReadFile(srcFS, srcPath)
	if err != nil {
		return err
	}
	err = fs.MkdirAll(dstFS, filepath.Dir(dstPath), 0755)
	if err != nil {
		return err
	}
	return fs.WriteFile(dstFS, dstPath, srcData, 0644)
}

func copyAllFS(dstFS fs.MutableFS, dstDir string, srcFS fs.FS, srcDir string) error {
	if err := fs.MkdirAll(dstFS, dstDir, 0755); err != nil {
		return err
	}
	return fs.WalkDir(srcFS, srcDir, fs.WalkDirFunc(func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			dstPath := filepath.Join(dstDir, strings.TrimPrefix(path, srcDir))
			return copyFileAcrossFS(dstFS, dstPath, srcFS, path)
		}
		return nil
	}))
}

// Avoids copying the bin directory
func (s *Service) copyKernelSource(dstDir string, srcFS fs.FS) error {
	if err := fs.MkdirAll(s.fsys, dstDir, 0755); err != nil {
		return err
	}
	return fs.WalkDir(srcFS, ".", fs.WalkDirFunc(func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path == "bin" {
			return fs.SkipDir
		}
		if !d.IsDir() {
			dstPath := filepath.Join(dstDir, strings.TrimPrefix(path, "."))
			return copyFileAcrossFS(s.fsys, dstPath, srcFS, path)
		}
		return nil
	}))
}

func (s *Service) copyFromInitFS(dst, src string) error {
	initFile := js.Global().Get("initfs").Get(src)
	if initFile.IsUndefined() {
		return nil
	}

	var exists bool
	fi, err := fs.Stat(s.fsys, dst)
	if err == nil {
		exists = true
	} else if os.IsNotExist(err) {
		exists = false
	} else {
		return err
	}

	if !exists || time.UnixMilli(int64(initFile.Get("mtimeMs").Float())).After(fi.ModTime()) {
		blob := initFile.Get("blob")
		buffer, err := jsutil.AwaitErr(blob.Call("arrayBuffer"))
		if err != nil {
			return err
		}

		// TODO: creating the file and applying the blob directly in indexedfs would be faster.
		data := make([]byte, blob.Get("size").Int())
		js.CopyBytesToGo(data, js.Global().Get("Uint8Array").New(buffer))
		err = fs.WriteFile(s.fsys, dst, data, 0644)
		if err != nil {
			return err
		}
	}

	return nil
}
