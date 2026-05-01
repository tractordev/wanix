// Package compatstat implements a minimal stat(1) using os.Stat.
package compatstat

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/u-root/u-root/pkg/core"
	"github.com/u-root/u-root/pkg/uroot/unixflag"
)

type command struct {
	core.Base
}

// New constructs the stat utility.
func New() core.Command {
	c := &command{}
	c.Init()
	return c
}

func (c *command) Run(args ...string) error {
	return c.RunContext(context.Background(), args...)
}

func fileKind(mode os.FileMode) string {
	switch {
	case mode&os.ModeDir != 0:
		return "directory"
	case mode&os.ModeSymlink != 0:
		return "symbolic link"
	case mode&os.ModeNamedPipe != 0:
		return "fifo"
	case mode&os.ModeSocket != 0:
		return "socket"
	case mode.IsRegular():
		return "regular file"
	default:
		return mode.Type().String()
	}
}

func (c *command) RunContext(ctx context.Context, args ...string) error {
	_ = ctx
	fs := flag.NewFlagSet("stat", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: stat FILE...\n")
		fmt.Fprintf(fs.Output(), "Report file status (summary).\n")
	}
	if err := fs.Parse(unixflag.ArgsToGoArgs(args)); err != nil {
		return err
	}
	names := fs.Args()
	if len(names) == 0 {
		fs.Usage()
		return fmt.Errorf("no file specified")
	}
	var errs int
	for _, name := range names {
		path := c.ResolvePath(name)
		fi, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(c.Stderr, "%s: %v\n", name, err)
			errs++
			continue
		}
		mode := fi.Mode()
		fmt.Fprintf(c.Stdout, "  File: %s\n", name)
		fmt.Fprintf(c.Stdout, "  Size: %d\n", fi.Size())
		fmt.Fprintf(c.Stdout, "  Kind: %s\n", fileKind(mode))
		fmt.Fprintf(c.Stdout, " Perms: %s\n", mode.String())
		if mt := fi.ModTime(); !mt.IsZero() {
			fmt.Fprintf(c.Stdout, "Modify: %s\n", mt.String())
		}
		fmt.Fprintln(c.Stdout)
	}
	if errs > 0 {
		return fmt.Errorf("%d path(s) could not be stated", errs)
	}
	return nil
}
