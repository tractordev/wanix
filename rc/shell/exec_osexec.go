//go:build !(js && wasm)

package shell

import (
	"context"
	"errors"
	"os/exec"

	"mvdan.cc/sh/v3/interp"
)

func runExternalCommand(ctx context.Context, hc interp.HandlerContext, path string, args []string) (int, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = hc.Stdin
	cmd.Stdout = hc.Stdout
	cmd.Stderr = hc.Stderr
	cmd.Dir = hc.Dir
	cmd.Env = exportedEnvPairs(hc.Env)

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}
