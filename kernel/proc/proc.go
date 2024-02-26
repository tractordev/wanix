package proc

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall/js"

	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/wanix/internal/jsutil"
	kfs "tractor.dev/wanix/kernel/fs"
)

type Service struct {
	FS      *kfs.Service
	nextPID int
	running map[int]*Process
	mu      sync.Mutex
}

func (s *Service) Initialize() {
	s.running = make(map[int]*Process)
}

func (s *Service) Get(pid int) (*Process, error) {
	s.mu.Lock()
	p, ok := s.running[pid]
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("no running process with PID %d", pid)
	}
	return p, nil
}

func (s *Service) Spawn(path string, args []string, env map[string]string, dir string) (*Process, error) {
	// TODO: check path exists, execute bit

	// can't use jsutil.WanixSyscall inside the kernel
	stat, err := jsutil.AwaitErr(js.Global().Get("api").Get("fs").Call("stat", path))
	if err != nil {
		return nil, err
	}

	if stat.Get("isDirectory").Bool() {
		matches, _ := fs.Glob(os.DirFS(unixToFsPath(path)), "*.go")
		if matches != nil && len(matches) > 0 {
			path, err = s.buildCmdSource(path, dir)
			if err != nil {
				return nil, err
			}
			if path == "" {
				return nil, os.ErrInvalid
			}
		}
	}

	if env == nil {
		// TODO: set from os.Environ()
	}

	if dir == "" {
		dir = "/"
	}

	s.mu.Lock()
	s.nextPID++
	p := &Process{
		ID:   s.nextPID,
		Path: path,
		Args: args,
		Env:  env,
		Dir:  dir,
	}
	s.running[s.nextPID] = p
	s.mu.Unlock()

	p.Task = js.Global().Get("task").Get("Task").New(js.Global().Get("initfs"), p.ID)
	_, err = jsutil.AwaitErr(p.Task.Call("exec", p.Path, jsutil.ToJSArray(p.Args), map[string]any{
		"env": jsutil.ToJSMap(p.Env),
		"dir": p.Dir,
	}))
	if err != nil {
		return nil, err
	}

	return p, nil
}

type Process struct {
	ID   int
	Task js.Value

	Path string
	Args []string
	Env  map[string]string
	Dir  string
}

func (p *Process) Stdout() io.ReadCloser {
	ch := jsutil.Await(p.Task.Call("stdout"))
	return &jsutil.Reader{ch}
}

func (p *Process) Stderr() io.ReadCloser {
	ch := jsutil.Await(p.Task.Call("stderr"))
	return &jsutil.Reader{ch}
}

func (p *Process) Output() io.ReadCloser {
	ch := jsutil.Await(p.Task.Call("output"))
	return &jsutil.Reader{ch}
}

func (p *Process) Stdin() io.WriteCloser {
	ch := jsutil.Await(p.Task.Call("stdin"))
	return &jsutil.Writer{ch}
}

func (p *Process) Terminate() error {
	_, err := jsutil.AwaitErr(p.Task.Call("terminate"))
	return err
}

func (p *Process) Wait() (int, error) {
	v, err := jsutil.AwaitErr(p.Task.Call("wait"))
	return v.Int(), err
}

func unixToFsPath(path string) string {
	return filepath.Clean(strings.TrimLeft(path, "/"))
}

// returns an empty wasmPath on error or non-zero exit code
func (s *Service) buildCmdSource(path, workingDir string) (wasmPath string, err error) {
	wasmPath = filepath.Join("/sys/bin", filepath.Base(path)+".wasm")

	dfs := os.DirFS("/")
	var wasmExists bool
	wasmStat, err := fs.Stat(dfs, unixToFsPath(wasmPath))
	if err == nil {
		wasmExists = true
	} else if os.IsNotExist(err) {
		wasmExists = false
	} else {
		return "", err
	}

	var shouldBuild bool
	if !wasmExists {
		shouldBuild = true
	} else {
		wasmMtime := wasmStat.ModTime()
		err = fs.WalkDir(dfs, unixToFsPath(path), func(walkPath string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			fi, err := d.Info()
			if err != nil {
				return err
			}

			if fi.ModTime().After(wasmMtime) {
				shouldBuild = true
				return fs.SkipAll
			}

			return nil
		})
		if err != nil {
			return "", err
		}
	}

	if shouldBuild {
		if path == "sys/cmd/shell" {
			jsutil.Log("Building Shell...")
		}
		p, err := s.Spawn(
			"/sys/cmd/build.wasm",
			[]string{"-output", wasmPath, path},
			map[string]string{},
			workingDir,
		)
		if err != nil {
			return "", err
		}

		// TODO: https://github.com/tractordev/wanix/issues/69
		// go io.Copy(os.Stdout, p.Stdout())
		// go io.Copy(os.Stderr, p.Stderr())

		exitCode, err := p.Wait()
		if exitCode != 0 {
			return "", err
		}
	}

	return wasmPath, nil
}
