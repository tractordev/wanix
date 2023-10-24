package proc

import (
	"io"
	"os"
	"sync"
	"syscall/js"

	"tractor.dev/wanix/internal/jsutil"
)

type Service struct {
	nextPID int
	running map[int]*Process
	mu      sync.Mutex
}

func (s *Service) Initialize() {
	s.running = make(map[int]*Process)
}

func (s *Service) InitializeJS() {
	// expose to subtasks
	js.Global().Get("api").Set("proc", map[string]any{
		"spawn": js.FuncOf(s.spawn),
	})
}

func (s *Service) spawn(this js.Value, args []js.Value) any {
	var (
		path  = args[0].String()
		args_ = jsutil.ToGoStringSlice(args[1])
		env   = jsutil.ToGoStringMap(args[2])
		dir   = args[3].String()
	)
	s.Spawn(path, args_, env, dir)
	return nil
}

func (s *Service) Spawn(path string, args []string, env map[string]string, dir string) *Process {
	initfs := jsutil.CopyObj(js.Global().Get("initfs"))
	if !jsutil.HasProp(initfs, path) {
		// TODO: read from osfs into new blob
	}

	if env == nil {
		// TODO: set from os.Environ()
	}

	if dir == "" {
		dir, _ = os.Getwd()
	}

	s.mu.Lock()
	s.nextPID++
	p := &Process{
		PID:  s.nextPID,
		Path: path,
		Args: args,
		Env:  env,
		Dir:  dir,
	}
	s.running[s.nextPID] = p
	s.mu.Unlock()

	t := js.Global().Get("task").Get("Task").New(initfs, p.PID)
	jsutil.Await(t.Call("init", p.Path, jsutil.ToJSArray(p.Args), map[string]any{
		"env": jsutil.ToJSMap(p.Env),
		"dir": p.Dir,
	}))

	p.Task = t
	p.Worker = t.Get("worker")

	return p
}

type Process struct {
	PID    int
	Worker js.Value
	Task   js.Value

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

func (p *Process) Kill() error {
	// todo
	return nil
}

func (p *Process) Wait() error {
	// todo
	return nil
}
