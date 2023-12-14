// exec is a cheap knock-off of os/exec to be used by userland programs
// to spawn subprocesses with the wanix kernel. Perhaps with WASI support
// it will be unnecessary as the os/exec can be used directly instead.
package exec

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"

	"tractor.dev/wanix/internal/jsutil"
)

type Cmd struct {
	Path string
	Args []string
	Env  []string
	Dir  string

	stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	PID int
}

func Command(name string, arg ...string) *Cmd {
	wd, _ := os.Getwd()
	return &Cmd{
		Path: name,
		Args: append([]string{name}, arg...),
		Env:  os.Environ(),
		Dir:  wd,
	}
}

func (c *Cmd) StdinPipe() io.WriteCloser {
	pr, pw := io.Pipe()
	c.stdin = pr
	return pw
}

func (c *Cmd) Start() error {
	menv := map[string]any{}
	for _, kvp := range c.Env {
		parts := strings.Split(kvp, "=")
		menv[parts[0]] = parts[1]
	}
	value, err := jsutil.WanixSyscall("proc.spawn", c.Path, jsutil.ToJSArray(c.Args[1:]), menv, c.Dir)
	if err != nil {
		return err
	}
	c.PID = value.Int()

	if c.stdin != nil {
		resp, err := jsutil.WanixSyscallResp("proc.stdin", c.PID)
		if err != nil {
			// TODO: cancel/kill process
			return err
		}
		go io.Copy(&jsutil.Writer{resp.Get("channel")}, c.stdin)
	}
	if c.Stdout != nil {
		resp, err := jsutil.WanixSyscallResp("proc.stdout", c.PID)
		if err != nil {
			// TODO: cancel/kill process
			return err
		}
		go io.Copy(c.Stdout, &jsutil.Reader{resp.Get("channel")})
	}
	if c.Stderr != nil {
		resp, err := jsutil.WanixSyscallResp("proc.stderr", c.PID)
		if err != nil {
			// TODO: cancel/kill process
			return err
		}
		go io.Copy(c.Stderr, &jsutil.Reader{resp.Get("channel")})
	}
	return nil
}

func (c *Cmd) Wait() (int, error) {
	value, err := jsutil.WanixSyscall("proc.wait", c.PID)
	if err != nil {
		return -1, err
	}
	return value.Int(), nil
}

func (c *Cmd) Run() (int, error) {
	if err := c.Start(); err != nil {
		return -1, err
	}
	return c.Wait()
}

func (c *Cmd) CombinedOutput() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if c.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	_, err := c.Run()
	return b.Bytes(), err
}
