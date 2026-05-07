package native

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"tractor.dev/wanix"
	"tractor.dev/wanix/fs"
)

// ExecDriver runs the task command as a host OS process (os/exec).
// Register it on the root task (e.g. root.Register("exec", &native.ExecDriver{}))
// or rely on Check for #task/new/auto when registered after js/wasm drivers.
type ExecDriver struct{}

var _ wanix.TaskDriver = (*ExecDriver)(nil)

func (d *ExecDriver) Check(t *wanix.Task) bool {
	arg0 := t.Arg(0)
	if arg0 == "" {
		return false
	}
	if strings.HasSuffix(arg0, ".wasm") || strings.HasSuffix(arg0, ".js") {
		return false
	}
	return true
}

func (d *ExecDriver) Start(t *wanix.Task) error {
	args := splitCmd(t.Cmd())
	if len(args) == 0 || args[0] == "" {
		return fmt.Errorf("task cmd is empty")
	}

	// stdinFile, _, err := t.FD(0)
	// if err != nil {
	// 	return fmt.Errorf("open task fd 0: %w", err)
	// }
	// stdoutFile, _, err := t.FD(1)
	// if err != nil {
	// 	return fmt.Errorf("open task fd 1: %w", err)
	// }
	// stderrFile, _, err := t.FD(2)
	// if err != nil {
	// 	return fmt.Errorf("open task fd 2: %w", err)
	// }

	// stdoutW, ok := stdoutFile.(io.Writer)
	// if !ok {
	// 	return fmt.Errorf("task fd 1 is not writable")
	// }
	// stderrW, ok := stderrFile.(io.Writer)
	// if !ok {
	// 	return fmt.Errorf("task fd 2 is not writable")
	// }

	cmd := exec.CommandContext(t.Context(), args[0], args[1:]...)
	if dir := strings.TrimSpace(t.Dir()); dir != "" {
		cmd.Dir = dir
	}
	if env := t.Env(); len(env) > 0 {
		cmd.Env = env
	}
	// cmd.Stdin = stdinFile
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}
	wanix.SetWorker(t, cmd.Process)

	go d.waitAndRecordExit(t, cmd)
	return nil
}

func (d *ExecDriver) waitAndRecordExit(t *wanix.Task, cmd *exec.Cmd) {
	err := cmd.Wait()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			code = 1
		}
	}
	f, err := t.Open("exit")
	if err != nil {
		return
	}
	_, _ = fs.Write(f, []byte(strconv.Itoa(code)))
	_ = f.Close()
}

// splitCmd mirrors wanix.Task.Arg (space-separated) but drops empty tokens so
// multiple spaces do not become bogus argv entries.
func splitCmd(cmd string) []string {
	if strings.TrimSpace(cmd) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(cmd, " ") {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
