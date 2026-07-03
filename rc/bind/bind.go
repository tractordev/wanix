package bind

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/u-root/u-root/pkg/core"
)

const selfBindsPath = "#task/self/binds"

type command struct {
	core.Base
}

// New creates a new bind command.
func New() core.Command {
	c := &command{}
	c.Init()
	return c
}

func (c *command) Run(args ...string) error {
	return c.RunContext(context.Background(), args...)
}

func (c *command) RunContext(ctx context.Context, args ...string) error {
	_ = ctx

	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		c.usage()
		return nil
	}

	if len(args) == 0 {
		return c.listBinds()
	}

	return WriteSelfCtl("bind " + strings.Join(args, " "))
}

func (c *command) listBinds() error {
	f, err := os.Open(selfBindsPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(c.Stdout, f)
	return err
}

func (c *command) usage() {
	fmt.Fprintf(c.Stderr, "Usage: bind [OPTIONS] SRC DST\n\n")
	fmt.Fprintf(c.Stderr, "With no arguments, list bindings from %s.\n", selfBindsPath)
	fmt.Fprintf(c.Stderr, "Otherwise write a bind control message to %s.\n\n", SelfCtlPath)
}
