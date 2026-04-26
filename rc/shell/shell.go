package shell

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// Main runs the rc shell and returns its process exit code.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("rc", flag.ContinueOnError)
	flags.SetOutput(stderr)
	command := flags.String("c", "", "shell command to run")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	wd, err := os.Getwd()
	if err != nil {
		fatalf(stderr, "rc: %v\n", err)
		return 1
	}
	env := expand.ListEnviron(os.Environ()...)

	runner, err := interp.New(
		interp.Dir(wd),
		interp.Env(env),
		interp.StdIO(stdin, stdout, stderr),
		interp.Interactive(*command == "" && flags.NArg() == 0),
		interp.CallHandler(helpCallHandler()),
		interp.ExecHandlers(urootCoreutilsMiddleware()),
		interp.ExecHandlers(wanixExecMiddleware()),
	)
	if err != nil {
		fatalf(stderr, "rc: %v\n", err)
		return 1
	}

	parser := syntax.NewParser(syntax.KeepComments(true))
	ctx := context.Background()

	switch {
	case *command != "":
		if err := runSource(ctx, runner, parser, "-c", strings.NewReader(*command)); err != nil {
			return exitCodeForErr(stderr, err)
		}
	case flags.NArg() > 0:
		file := flags.Arg(0)
		f, err := os.Open(file)
		if err != nil {
			fatalf(stderr, "rc: %v\n", err)
			return 1
		}
		defer f.Close()
		if err := runSource(ctx, runner, parser, file, f); err != nil {
			return exitCodeForErr(stderr, err)
		}
	default:
		if err := runREPL(ctx, runner, parser, stdin, stderr); err != nil {
			return exitCodeForErr(stderr, err)
		}
	}
	return 0
}

func runREPL(ctx context.Context, r *interp.Runner, parser *syntax.Parser, stdin io.Reader, stderr io.Writer) error {
	scanner := bufio.NewScanner(stdin)
	for {
		fmt.Fprint(stderr, "rc% ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			return nil
		}
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if err := runSource(ctx, r, parser, "<stdin>", strings.NewReader(line+"\n")); err != nil {
			var status interp.ExitStatus
			if errors.As(err, &status) {
				fmt.Fprintf(stderr, "exit status %d\n", status)
				continue
			}
			return err
		}
		if r.Exited() {
			return nil
		}
	}
}

func runSource(ctx context.Context, r *interp.Runner, parser *syntax.Parser, name string, src io.Reader) error {
	prog, err := parser.Parse(src, name)
	if err != nil {
		return err
	}
	return r.Run(ctx, prog)
}

func wanixExecMiddleware() func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			hc := interp.HandlerCtx(ctx)
			if len(args) == 0 {
				return next(ctx, args)
			}
			path, err := resolveExecPath(hc, args[0])
			if err != nil {
				fmt.Fprintf(hc.Stderr, "rc: %s: %v\n", args[0], err)
				return interp.ExitStatus(127)
			}
			warnIfNotExecutable(hc, path)

			code, err := runExternalCommand(ctx, hc, path, args[1:])
			if err != nil {
				return err
			}
			if code == 0 {
				return nil
			}
			return interp.ExitStatus(code)
		}
	}
}

func resolveExecPath(hc interp.HandlerContext, cmd string) (string, error) {
	if filepath.IsAbs(cmd) || strings.HasPrefix(cmd, "./") || strings.HasPrefix(cmd, "../") {
		if _, err := os.Stat(cmd); err != nil {
			return "", err
		}
		return cmd, nil
	}

	pathVar := hc.Env.Get("PATH")
	pathEntries := []string{""}
	if pathVar.IsSet() && pathVar.Str != "" {
		pathEntries = strings.Split(pathVar.Str, ":")
	}

	candidates := make([]string, 0, len(pathEntries)+1)
	if strings.Contains(cmd, "/") {
		// Plan 9-like behavior: allow subpaths in PATH lookups.
		for _, base := range pathEntries {
			if base == "" {
				candidates = append(candidates, cmd)
				continue
			}
			candidates = append(candidates, filepath.Join(base, cmd))
		}
		candidates = append(candidates, cmd)
	} else {
		for _, base := range pathEntries {
			if base == "" {
				candidates = append(candidates, cmd)
				continue
			}
			candidates = append(candidates, filepath.Join(base, cmd))
		}
	}

	for _, candidate := range candidates {
		full := candidate
		if !filepath.IsAbs(full) {
			full = filepath.Join(hc.Dir, full)
		}
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		return candidate, nil
	}
	return "", fs.ErrNotExist
}

func warnIfNotExecutable(hc interp.HandlerContext, path string) {
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(hc.Dir, path)
	}
	info, err := os.Stat(full)
	if err != nil {
		return
	}
	if info.Mode()&0o111 == 0 {
		fmt.Fprintf(hc.Stderr, "rc: warning: %s is not marked executable\n", path)
	}
}

func exportedEnvPairs(env expand.Environ) []string {
	pairs := make([]string, 0, 64)
	env.Each(func(name string, vr expand.Variable) bool {
		if vr.Exported && vr.IsSet() && vr.Kind == expand.String {
			pairs = append(pairs, name+"="+vr.Str)
		}
		return true
	})
	return pairs
}

func printHelp(hc interp.HandlerContext) {
	builtins := []string{"help", "cd", "pwd", "echo", "exit", "export", "unset", "type", ":", "true", "false"}
	embedded := bundledCommandNames()

	fmt.Fprintln(hc.Stdout, "rc help")
	fmt.Fprintln(hc.Stdout, "")
	fmt.Fprintln(hc.Stdout, "Usage:")
	fmt.Fprintln(hc.Stdout, "  help")
	fmt.Fprintln(hc.Stdout, "  help <command-or-builtin>")
	fmt.Fprintln(hc.Stdout, "")
	fmt.Fprintf(hc.Stdout, "Builtins: %s\n", strings.Join(builtins, ", "))
	fmt.Fprintf(hc.Stdout, "Bundled commands: %s\n", strings.Join(embedded, ", "))
	fmt.Fprintln(hc.Stdout, "External commands: resolved via PATH lookup")
}

func helpCallHandler() interp.CallHandlerFunc {
	return func(ctx context.Context, args []string) ([]string, error) {
		if len(args) > 0 && args[0] == "help" {
			if len(args) > 2 {
				return nil, fmt.Errorf("help: usage: help [command-or-builtin]")
			}
			if len(args) == 2 {
				return []string{args[1], "--help"}, nil
			}
			printHelp(interp.HandlerCtx(ctx))
			return []string{":"}, nil
		}
		return args, nil
	}
}

func exitCodeForErr(stderr io.Writer, err error) int {
	var status interp.ExitStatus
	if errors.As(err, &status) {
		return int(status)
	}
	fatalf(stderr, "rc: %v\n", err)
	return 1
}

func fatalf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
