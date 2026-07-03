package write

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/u-root/u-root/pkg/core"
	"github.com/u-root/u-root/pkg/uroot/unixflag"
	"mvdan.cc/sh/v3/interp"
)

type command struct {
	core.Base
}

// New creates a new write command.
func New() core.Command {
	c := &command{}
	c.Init()
	return c
}

func (c *command) Run(args ...string) error {
	return c.RunContext(context.Background(), args...)
}

func (c *command) RunContext(ctx context.Context, args ...string) error {
	var appendMode bool

	fs := flag.NewFlagSet("write", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	fs.BoolVar(&appendMode, "a", false, "append instead of truncating")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: write [-a] FILENAME [ARG ...]\n\n")
		fmt.Fprintf(fs.Output(), "Writes ARGs as a newline-terminated line to FILENAME.\n")
		fmt.Fprintf(fs.Output(), "With no arguments, copies stdin to FILENAME.\n\n")
	}

	if err := fs.Parse(unixflag.ArgsToGoArgs(args)); err != nil {
		return err
	}

	pos := fs.Args()
	if len(pos) < 1 {
		fs.Usage()
		return fmt.Errorf("no filename specified")
	}

	flags := os.O_WRONLY | os.O_CREATE
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	path := c.ResolvePath(pos[0])
	fd, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		return err
	}
	defer fd.Close()

	if len(pos) > 1 {
		line := strings.Join(pos[1:], " ") + "\n"
		_, err = fd.Write([]byte(line))
		return err
	}

	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(fd, c.Stdin)
		done <- err
	}()

	select {
	case <-ctx.Done():
		_ = fd.Close()
		return interp.ExitStatus(130)
	case err := <-done:
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
	}
	return nil
}
