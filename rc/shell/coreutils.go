package shell

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/u-root/u-root/pkg/core"
	"github.com/u-root/u-root/pkg/core/base64"
	"github.com/u-root/u-root/pkg/core/cat"
	"github.com/u-root/u-root/pkg/core/chmod"
	"github.com/u-root/u-root/pkg/core/cp"
	"github.com/u-root/u-root/pkg/core/mkdir"
	"github.com/u-root/u-root/pkg/core/mv"
	"github.com/u-root/u-root/pkg/core/rm"
	"github.com/u-root/u-root/pkg/core/shasum"
	"github.com/u-root/u-root/pkg/core/tar"
	"github.com/u-root/u-root/pkg/core/touch"
	"github.com/u-root/u-root/pkg/core/xargs"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"tractor.dev/wanix/rc/shell/compatfind"
	"tractor.dev/wanix/rc/shell/compatgzip"
	"tractor.dev/wanix/rc/shell/compatls"
)

var coreutilsCommands = map[string]func() core.Command{
	"cat":    func() core.Command { return cat.New() },
	"tar":    func() core.Command { return tar.New() },
	"touch":  func() core.Command { return touch.New() },
	"shasum": func() core.Command { return shasum.New() },
	"xargs":  func() core.Command { return xargs.New() },
	"chmod":  func() core.Command { return chmod.New() },
	"base64": func() core.Command { return base64.New() },
	"mkdir":  func() core.Command { return mkdir.New() },
	"cp":     func() core.Command { return cp.New() },
	"mv":     func() core.Command { return mv.New() },
	"rm":     func() core.Command { return rm.New() },
	"ls":     func() core.Command { return compatls.New() },
	"find":   func() core.Command { return compatfind.New() },
	"gzip":   func() core.Command { return compatgzip.New("gzip") },
	"gzcat":  func() core.Command { return compatgzip.New("gzcat") },
	"gunzip": func() core.Command { return compatgzip.New("gunzip") },
}

func urootCoreutilsMiddleware() func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return next(ctx, args)
			}
			if args[0] == "env" {
				return runEnvCommand(ctx, args[1:], next)
			}

			newCmd, ok := coreutilsCommands[args[0]]
			if !ok {
				return next(ctx, args)
			}

			hc := interp.HandlerCtx(ctx)
			cmd := newCmd()
			cmd.SetIO(hc.Stdin, hc.Stdout, hc.Stderr)
			cmd.SetWorkingDir(hc.Dir)
			cmd.SetLookupEnv(func(key string) (string, bool) {
				v := hc.Env.Get(key)
				return v.Str, v.Set
			})

			if err := cmd.RunContext(ctx, args[1:]...); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				fmt.Fprintf(hc.Stderr, "rc: %s: %v\n", args[0], err)
				return interp.ExitStatus(1)
			}
			return nil
		}
	}
}

func bundledCommandNames() []string {
	names := make([]string, 0, len(coreutilsCommands)+1)
	for name := range coreutilsCommands {
		names = append(names, name)
	}
	names = append(names, "env")
	sort.Strings(names)
	return names
}

func runEnvCommand(ctx context.Context, args []string, next interp.ExecHandlerFunc) error {
	hc := interp.HandlerCtx(ctx)
	env := map[string]string{}
	hc.Env.Each(func(name string, vr expand.Variable) bool {
		// We intentionally include all set vars available via interp env.
		if vr.IsSet() {
			env[name] = vr.Str
		}
		return true
	})

	clear := false
	i := 0
	for ; i < len(args); i++ {
		arg := args[i]
		if arg == "-i" {
			clear = true
			continue
		}
		if strings.HasPrefix(arg, "-") && !strings.Contains(arg, "=") {
			return fmt.Errorf("unsupported flag %s", arg)
		}
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			env[parts[0]] = parts[1]
			continue
		}
		break
	}

	if clear {
		onlyAssigned := map[string]string{}
		for j := 0; j < i; j++ {
			if strings.Contains(args[j], "=") {
				parts := strings.SplitN(args[j], "=", 2)
				onlyAssigned[parts[0]] = parts[1]
			}
		}
		env = onlyAssigned
	}

	if i < len(args) {
		// No env mutation on command execution yet; pass through plain `env cmd`.
		if !clear {
			hasAssignments := false
			for _, a := range args[:i] {
				if strings.Contains(a, "=") {
					hasAssignments = true
					break
				}
			}
			if !hasAssignments {
				return next(ctx, args[i:])
			}
		}
		return fmt.Errorf("command execution with modified env is not supported yet")
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(hc.Stdout, "%s=%s\n", k, env[k])
	}
	return nil
}
