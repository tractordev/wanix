package openfile

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/u-root/u-root/pkg/core"
	"github.com/u-root/u-root/pkg/uroot/unixflag"
	"mvdan.cc/sh/v3/interp"
)

// command implements the openfile core utility.
type command struct {
	core.Base
}

// New creates a new openfile command.
func New() core.Command {
	c := &command{}
	c.Init()
	return c
}

// Run executes the command with a `context.Background()`.
func (c *command) Run(args ...string) error {
	return c.RunContext(context.Background(), args...)
}

func isJsInvokeProbe(fi os.FileInfo) bool {
	m := fi.Mode()
	return !m.IsDir() && m.IsRegular() && fi.Size() == 0 && m.Perm()&0111 != 0
}

// RunContext runs openfile with the given ctx.
//
// It streams the opened file to stdout until read EOF. Ordinary files EOF after their content.
// Invoke-style JS function files (executable+length 0 Stat) accept a single stdin line once, then
// copy the invoke result to stdout until that read stream EOFs (one-shot).
func (c *command) RunContext(ctx context.Context, args ...string) error {
	fs := flag.NewFlagSet("openfile", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: openfile FILENAME\n\n")
		fmt.Fprintf(fs.Output(), "Opens the file for read+write and copies its readable side to stdout until EOF.\n")
		fmt.Fprintf(fs.Output(), "Invoke-style files consume newline-terminated stdin to run each invocation.\n\n")
	}

	if err := fs.Parse(unixflag.ArgsToGoArgs(args)); err != nil {
		return err
	}

	if len(fs.Args()) < 1 {
		fs.Usage()
		return fmt.Errorf("no filename specified")
	}

	path := c.ResolvePath(fs.Arg(0))
	fd, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer fd.Close()

	fi, err := fd.Stat()
	if err != nil {
		return err
	}

	if isJsInvokeProbe(fi) {
		sc := bufio.NewScanner(c.Stdin)
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				return err
			}
			return nil // EOF before first line → nothing to invoke
		}
		line := append([]byte(sc.Text()), '\n')
		if _, err := fd.Write(line); err != nil {
			return err
		}
		if err := sc.Err(); err != nil {
			return err
		}
	}

	copyDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(c.Stdout, fd)
		copyDone <- err
	}()

	var copyErr error
	select {
	case <-ctx.Done():
		_ = fd.Close()
		return interp.ExitStatus(130)
	case copyErr = <-copyDone:
	}

	_ = fd.Close()

	if copyErr != nil && !errors.Is(copyErr, io.EOF) {
		return copyErr
	}
	return nil
}
