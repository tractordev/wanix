package fs

import (
	"archive/tar"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
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
		jsutil.Log(args...)
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

	hostURL := js.Global().Get("hostURL").String()

	devURL := fmt.Sprintf("%ssys/dev", hostURL)
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

	if exists, _ := fs.Exists(s.fsys, "sys/initfs"); !exists {
		jsutil.Log("Loading initfs...")

		// Fetch wanix-initfs.gz
		initfsUrl := fmt.Sprintf("%swanix-initfs.gz", hostURL)
		resp, err := http.DefaultClient.Get(initfsUrl)
		if err != nil {
			panic(fmt.Sprintf("%s: Couldn't fetch initfs from %s", err.Error(), initfsUrl))
		}
		defer resp.Body.Close()

		jsutil.Log("GET", initfsUrl, resp.Status)
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			panic("ErrBadStatus: " + resp.Status)
		}

		// Decompress gzip and unpack tar to /sys/*
		zr, err := gzip.NewReader(resp.Body)
		must(err)
		defer zr.Close()

		tr := tar.NewReader(zr)
		var hdr *tar.Header
		var tarErr error
		for hdr, tarErr = tr.Next(); tarErr == nil; hdr, tarErr = tr.Next() {
			dstPath := filepath.Join("sys", hdr.Name)

			var exists bool
			fi, err := fs.Stat(s.fsys, dstPath)
			if err == nil {
				exists = true
			} else if os.IsNotExist(err) {
				exists = false
			} else {
				panic(err)
			}

			if !exists || hdr.ModTime.After(fi.ModTime()) {
				switch hdr.Typeflag {
				case tar.TypeDir:
					must(fs.MkdirAll(s.fsys, dstPath, 0755))
				case tar.TypeReg:
					jsutil.Log(fmt.Sprintf("Copying initfs/%s to %s (%d bytes)", filepath.Clean(hdr.Name), dstPath, hdr.Size))

					f, err := s.fsys.Create(dstPath)
					must(err)

					_, err = io.Copy(f.(io.WriteCloser), tr)
					must(err)
					must(f.Close())
				default:
					panic(fmt.Sprint("Unhandled tar Typeflag:", hdr.Typeflag))
				}
			}
		}
		if tarErr != io.EOF {
			panic(tarErr)
		}

		// Set mtime on shell.wasm to make sure we don't build the shell
		// on first boot even though we embedded a pre-compiled shell
		s.fsys.Chtimes("sys/bin/shell.wasm", time.Time{}, time.Now())

		// Leave signal that we've already unpacked initfs before,
		// speeding up subsequent boots whilst allowing the user to
		// trigger a fetch again by deleting the file.
		f, err := s.fsys.Create("sys/initfs")
		must(err)
		must(f.Close())
	}

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

func copyAllFS(dstFS fs.MutableFS, dstDir string, srcFS fs.FS, srcDir string) error {
	if err := fs.MkdirAll(dstFS, dstDir, 0755); err != nil {
		return err
	}
	return fs.WalkDir(srcFS, srcDir, fs.WalkDirFunc(func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			srcData, err := fs.ReadFile(srcFS, path)
			if err != nil {
				return err
			}
			dstPath := filepath.Join(dstDir, strings.TrimPrefix(path, srcDir))
			err = fs.MkdirAll(dstFS, filepath.Dir(dstPath), 0755)
			if err != nil {
				return err
			}
			return fs.WriteFile(dstFS, dstPath, srcData, 0644)
		}
		return nil
	}))
}
