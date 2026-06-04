//go:build js && wasm

package shell

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mvdan.cc/sh/v3/interp"
)

func readString(path string) (string, error) {
	out, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func runExternalCommand(ctx context.Context, hc interp.HandlerContext, path string, args []string) (int, error) {
	argv := append([]string{path}, args...)
	termID, err := readString("#term/new")
	if err != nil {
		return 1, err
	}
	termPath := filepath.Join("#term", termID)

	// todo: need better auto so we dont have to use gojs here
	taskID, err := readString("#task/new/gojs")
	if err != nil {
		return 1, err
	}
	taskPath := filepath.Join("#task", taskID)

	// todo: these should be os.WriteFile but truncating synthetic files isnt allowed yet
	if err := AppendFile(filepath.Join(taskPath, "cmd"), []byte(joinTaskArgs(argv))); err != nil {
		return 1, err
	}
	if err := AppendFile(filepath.Join(taskPath, "env"), []byte(joinTaskEnv(hc))); err != nil {
		return 1, err
	}
	if err := AppendFile(filepath.Join(taskPath, "dir"), []byte(hc.Dir)); err != nil {
		return 1, err
	}

	ctlmsg := []string{
		fmt.Sprintf("bind %s/program %s/fd/0", termPath, taskPath),
		fmt.Sprintf("bind %s/program %s/fd/1", termPath, taskPath),
		fmt.Sprintf("bind %s/program %s/fd/2", termPath, taskPath),
	}
	for _, msg := range ctlmsg {
		if err := AppendFile(filepath.Join(taskPath, "ctl"), []byte(msg)); err != nil {
			return 1, err
		}
	}

	termData, err := os.Open(filepath.Join(termPath, "data"))
	if err != nil {
		return 1, err
	}
	defer termData.Close()

	if shouldForwardStdin(hc.Stdin) {
		// todo: do we need to do line discpline?
		go func() {
			println("copying stdin")
			_, _ = io.Copy(termData, hc.Stdin)
			println("stdin done")
		}()
	}

	go func() {
		println("copying stdout")
		_, _ = io.Copy(hc.Stdout, termData)
		println("stdout done")
	}()

	if err := AppendFile(filepath.Join(taskPath, "ctl"), []byte("start")); err != nil {
		return 1, err
	}

	code, err := waitExitCode(ctx, filepath.Join(taskPath, "exit"))
	if err != nil {
		return 1, err
	}

	return code, nil
}

func shouldForwardStdin(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return true
	}
	return f.Fd() != os.Stdin.Fd()
}

func waitExitCode(ctx context.Context, path string) (int, error) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return 1, ctx.Err()
		case <-ticker.C:
			out, err := os.ReadFile(path)
			if err != nil {
				return 1, err
			}
			out = []byte(strings.TrimSpace(string(out)))
			if len(out) == 0 {
				continue
			}
			code, err := strconv.Atoi(string(out))
			if err != nil {
				return 1, err
			}
			return code, nil
		}
	}
}

func joinTaskEnv(hc interp.HandlerContext) string {
	return strings.Join(append(exportedEnvPairs(hc.Env), ""), "\n")
}

func joinTaskArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = quoteArg(arg)
	}
	return strings.Join(quoted, " ")
}

func quoteArg(in string) string {
	if in == "" {
		return "''"
	}
	if !strings.ContainsAny(in, " \t\n'\"\\") {
		return in
	}
	return "'" + strings.ReplaceAll(in, "'", `'"'"'`) + "'"
}

func AppendFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
