package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/anmitsu/go-shlex"
	"github.com/spf13/afero"
	"golang.org/x/term"
	"tractor.dev/toolkit-go/engine"
	"tractor.dev/toolkit-go/engine/cli"
)

func main() {
	engine.Run(Shell{})
}

type Shell struct {
	Root *cli.Command
}

func (m *Shell) Initialize() {
	m.Root = &cli.Command{}
	m.Root.AddCommand(echoCmd())
	m.Root.AddCommand(openCmd())
	m.Root.AddCommand(mtimeCmd())
	m.Root.AddCommand(lsCmd())
	m.Root.AddCommand(cdCmd())
	m.Root.AddCommand(catCmd())
	m.Root.AddCommand(reloadCmd())
	m.Root.AddCommand(downloadCmd())
	m.Root.AddCommand(touchCmd())
	m.Root.AddCommand(removeCmd())
	m.Root.AddCommand(mkdirCmd())
	m.Root.AddCommand(moveCmd())
	m.Root.AddCommand(copyCmd())
	m.Root.AddCommand(pwdCmd())
	m.Root.AddCommand(writeCmd())
	m.Root.AddCommand(printEnvCmd())
	m.Root.AddCommand(exportCmd())
}

func (m *Shell) Run(ctx context.Context) error {

	io.WriteString(os.Stdout, `
      ____    _____  _____     ___    __      __   ____   _
  |  |    |  |    /  \    |    \  |  | (_    _) \  \  /  / 
  |  |    |  |   /    \   |  |\ \ |  |   |  |    \  \/  /  
  |  |    |  |  /  ()  \  |  | \ \|  |   |  |     >    <   
   \  \/\/  /  |   __   | |  |  \    |  _|  |_   /  /\  \  
  __\      /___|  (__)  |_|  |___\   |_(      )_/  /__\  \_
																	 
																	 
`)
	// if !js.Global().Get("account").IsUndefined() {
	// 	fmt.Fprintf(ch, "Signed in as %s.\n\n", js.Global().Get("account").Get("profile").Get("name").String())
	// }

	fsys := afero.NewOsFs()
	stdio := iorwc{
		ReadCloser:  os.Stdin,
		WriteCloser: os.Stdout,
	}

	t := term.NewTerminal(stdio, "/ ▶ ")
	// if err := t.SetSize(req.Cols, req.Rows); err != nil {
	// 	fmt.Println(err)
	// }

	m.Root.Run = func(ctx *cli.Context, args []string) {
		if exe, found, isScript := findExecutable(t, fsys, args[0], true); found {
			if isScript {
				runScript(stdio, t, fsys, exe, args[1:])
			} else {
				exit := runWasm(stdio, t, fsys, exeEnv, exe, args[1:])
				exit.check(t)
			}
		} else {
			io.WriteString(t, "command or executable not found\n")
		}
	}

	for {
		wd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		t.SetPrompt(fmt.Sprintf("%s ▶ ", wd))

		for _, kvp := range os.Environ() {
			parts := strings.SplitN(kvp, "=", 2)
			shellEnv[parts[0]] = parts[1]
		}

		l, err := t.ReadLine()
		if err != nil {
			log.Fatal(err)
		}
		if l == "" {
			continue
		}

		// this needs to be rethought, but is left here
		// to be re-implemented somewhere else later
		// --
		// if strings.HasPrefix(l, "@") && !js.Global().Get("wanix").Get("collab").IsUndefined() {
		// 	parts := strings.SplitN(l, " ", 2)
		// 	js.Global().Get("wanix").Get("jazzfs").Get("sendMessage").Invoke(strings.TrimPrefix(parts[0], "@"), parts[1])
		// 	continue
		// }

		args, err := shlex.Split(l, true)
		if err != nil {
			fmt.Fprintln(t, err)
			fmt.Fprintln(t)
			continue
		}

		// how will terminal notify size changes?
		// shell can explicitly listen for changes,
		// but others (micro) won't be able to...
		// --
		// shellEnv["LINES"] = strconv.Itoa(size.Rows)
		// shellEnv["COLUMNS"] = strconv.Itoa(size.Cols)

		// Setup child process environment
		ok, overrideEnv, args := parseEnvVars(t, args)
		if !ok {
			continue
		}
		if len(args) == 0 {
			io.WriteString(t, "missing command or executable\n")
			continue
		}
		if overrideEnv == nil {
			exeEnv = shellEnv
		} else {
			exeEnv = make(map[string]string)
			for k, v := range shellEnv {
				exeEnv[k] = v
			}
			for k, v := range overrideEnv {
				exeEnv[k] = v
			}
		}

		if err := cli.Execute(context.Background(), m.Root, args); err != nil {
			io.WriteString(t, err.Error())
		}

		io.WriteString(t, "\n")
	}
}

type iorwc struct {
	io.ReadCloser
	io.WriteCloser
}

func (io *iorwc) Close() error {
	return nil
}
