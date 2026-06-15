package attach

import (
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

type command struct {
	core.Base
}

// New creates a new attach command.
func New() core.Command {
	c := &command{}
	c.Init()
	return c
}

func (c *command) Run(args ...string) error {
	return c.RunContext(context.Background(), args...)
}

func (c *command) RunContext(ctx context.Context, args ...string) error {
	var attachStdin bool

	fs := flag.NewFlagSet("attach", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	fs.BoolVar(&attachStdin, "i", false, "also attach stdin to the file")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: attach [-i] FILENAME\n\n")
		fmt.Fprintf(fs.Output(), "Attaches the file's readable side to stdout.\n")
		fmt.Fprintf(fs.Output(), "With -i, also attaches stdin to the file in parallel.\n\n")
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

	done := make(chan error, 2)

	go func() {
		_, err := io.Copy(c.Stdout, fd)
		done <- err
	}()

	if attachStdin {
		go func() {
			_, err := io.Copy(fd, c.Stdin)
			done <- err
		}()
	}

	copies := 1
	if attachStdin {
		copies = 2
	}

	var copyErr error
	for range copies {
		select {
		case <-ctx.Done():
			_ = fd.Close()
			return interp.ExitStatus(130)
		case err := <-done:
			if err != nil && !errors.Is(err, io.EOF) {
				copyErr = err
			}
		}
	}

	_ = fd.Close()

	return copyErr
}
