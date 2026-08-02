package rc

import (
	"context"
	"io"

	"mvdan.cc/sh/v3/interp"
)

// commandExecutor runs a command through rc's normal dispatcher with the
// supplied standard streams.
type commandExecutor func(context.Context, []string, io.Reader, io.Writer, io.Writer) error

type execIO struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

type execIOKey struct{}

// withExecIO carries scoped streams through mvdan's ExecHandlerFunc. mvdan's
// HandlerContext key is private, so callers cannot replace it directly.
func withExecIO(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) context.Context {
	return context.WithValue(ctx, execIOKey{}, execIO{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	})
}

func ioForExec(ctx context.Context, hc interp.HandlerContext) (io.Reader, io.Writer, io.Writer) {
	if streams, ok := ctx.Value(execIOKey{}).(execIO); ok {
		return streams.stdin, streams.stdout, streams.stderr
	}
	return hc.Stdin, hc.Stdout, hc.Stderr
}
