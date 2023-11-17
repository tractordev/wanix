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
	"syscall/js"

	"tractor.dev/wanix/internal/jsutil"
)

type Cmd struct {
	Path string
	Args []string
	Env  []string
	Dir  string

	Stdin  io.Reader
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

func (c *Cmd) Start() error {
	menv := map[string]any{}
	for _, kvp := range c.Env {
		parts := strings.Split(kvp, "=")
		menv[parts[0]] = parts[1]
	}
	resp, err := jsutil.AwaitErr(js.Global().Get("sys").Call("call", "proc.spawn", []any{c.Path, jsutil.ToJSArray(c.Args[1:]), menv, c.Dir}))
	if err != nil {
		return err
	}

	c.PID = resp.Get("value").Int()

	if c.Stdin != nil {
		resp, err := jsutil.AwaitErr(js.Global().Get("sys").Call("call", "proc.stdin", []any{c.PID}))
		if err != nil {
			// TODO: cancel/kill process
			return err
		}
		go io.Copy(&jsutil.Writer{resp.Get("channel")}, c.Stdin)
	}
	if c.Stdout != nil {
		resp, err := jsutil.AwaitErr(js.Global().Get("sys").Call("call", "proc.stdout", []any{c.PID}))
		if err != nil {
			// TODO: cancel/kill process
			return err
		}
		go io.Copy(c.Stdout, &jsutil.Reader{resp.Get("channel")})
	}
	if c.Stderr != nil {
		resp, err := jsutil.AwaitErr(js.Global().Get("sys").Call("call", "proc.stderr", []any{c.PID}))
		if err != nil {
			// TODO: cancel/kill process
			return err
		}
		go io.Copy(c.Stderr, &jsutil.Reader{resp.Get("channel")})
	}
	return nil
}

func (c *Cmd) Wait() (int, error) {
	resp, err := jsutil.AwaitErr(js.Global().Get("sys").Call("call", "proc.wait", []any{c.PID}))
	if err != nil {
		return -1, err
	}
	return resp.Get("value").Int(), nil
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
