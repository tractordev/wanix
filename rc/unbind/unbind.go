package unbind

import (
	"context"
	"fmt"

	"github.com/u-root/u-root/pkg/core"
	"tractor.dev/wanix/rc/bind"
)

type command struct {
	core.Base
}

// New creates a new unbind command.
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

	if len(args) != 2 {
		c.usage()
		return fmt.Errorf("src and dst required")
	}

	return bind.WriteSelfCtl("unbind " + args[0] + " " + args[1])
}

func (c *command) usage() {
	fmt.Fprintf(c.Stderr, "Usage: unbind SRC DST\n\n")
	fmt.Fprintf(c.Stderr, "Write an unbind control message to %s.\n\n", bind.SelfCtlPath)
}
