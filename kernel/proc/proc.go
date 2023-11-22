package proc

import (
	"fmt"
	"io"
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

	if env == nil {
		// TODO: set from os.Environ()
	}

	if dir == "" {
		dir = "/"
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

	p.Task = js.Global().Get("task").Get("Task").New(js.Global().Get("initfs"), p.PID)
	_, err := jsutil.AwaitErr(p.Task.Call("exec", p.Path, jsutil.ToJSArray(p.Args), map[string]any{
		"env": jsutil.ToJSMap(p.Env),
		"dir": p.Dir,
	}))
	if err != nil {
		return nil, err
	}

	return p, nil
}

type Process struct {
	PID  int
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
