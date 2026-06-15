package invoke

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/u-root/u-root/pkg/core"
	"github.com/u-root/u-root/pkg/uroot/unixflag"
	"mvdan.cc/sh/v3/interp"
)

type command struct {
	core.Base
}

// New creates a new invoke command.
func New() core.Command {
	c := &command{}
	c.Init()
	return c
}

func (c *command) Run(args ...string) error {
	return c.RunContext(context.Background(), args...)
}

func (c *command) RunContext(ctx context.Context, args ...string) error {
	var timeout time.Duration

	fs := flag.NewFlagSet("invoke", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	fs.DurationVar(&timeout, "timeout", 30*time.Second, "max time to wait for output")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: invoke [-timeout duration] FILENAME [ARG ...]\n\n")
		fmt.Fprintf(fs.Output(), "Opens the file, writes a newline-terminated invocation line, then\n")
		fmt.Fprintf(fs.Output(), "copies the readable side to stdout until EOF or timeout.\n")
		fmt.Fprintf(fs.Output(), "With no arguments, only a newline is written.\n\n")
	}

	if err := fs.Parse(unixflag.ArgsToGoArgs(args)); err != nil {
		return err
	}

	pos := fs.Args()
	if len(pos) < 1 {
		fs.Usage()
		return fmt.Errorf("no filename specified")
	}

	line := "\n"
	if len(pos) > 1 {
		line = strings.Join(pos[1:], " ") + "\n"
	}

	path := c.ResolvePath(pos[0])
	fd, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer fd.Close()

	if _, err := fd.Write([]byte(line)); err != nil {
		return err
	}

	readCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		readCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(c.Stdout, fd)
		done <- err
	}()

	select {
	case <-readCtx.Done():
		_ = fd.Close()
		if errors.Is(readCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("read timed out after %s", timeout)
		}
		return interp.ExitStatus(130)
	case err := <-done:
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
	}
	return nil
}
