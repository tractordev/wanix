package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/anmitsu/go-shlex"
	"golang.org/x/term"
	"tractor.dev/toolkit-go/engine"
	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/kernel/proc/exec"
)

func main() {
	engine.Run(Shell{})
}

type Shell struct {
	Root       *cli.Command
	stdio      iorwc
	scriptMode bool
	lineNum    int
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

	m.stdio = iorwc{
		ReadCloser:  os.Stdin,
		WriteCloser: os.Stdout,
	}

	fmt.Println("Shell Args:", os.Args)

	if len(os.Args) > 1 {
		if filepath.Ext(os.Args[1]) != ".sh" {
			fmt.Println("Script argument must be a '.sh' file")
		} else {
			if f, err := os.Open(os.Args[1]); err != nil {
				fmt.Println("Couldn't open script:", err)
			} else {
				m.stdio.ReadCloser = f
				m.scriptMode = true
			}
		}
	}
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

	t := term.NewTerminal(m.stdio, "/ ▶ ")
	// if err := t.SetSize(req.Cols, req.Rows); err != nil {
	// 	fmt.Println(err)
	// }

	m.Root.Run = func(ctx *cli.Context, args []string) {
		if cmd := searchForCommand(args[0]); cmd.found {
			var exeCmd *exec.Cmd

			switch cmd.CmdType {
			case CmdIsScript:
				execArgs := append([]string{cmd.path}, args[1:]...)
				// TODO: shell is currently only available in the initfs,
				// but the process worker is able to exec it from there anyway.
				// We should really mount the shell exe in /sys/bin though.
				exeCmd = exec.Command("shell", execArgs...)

			case CmdIsSourceDir:
				if path, err := buildCmdSource(cmd.path); checkErr(t, err) {
					m.ifScriptPrintErr(t)
					return
				} else {
					exeCmd = exec.Command(path, args[1:]...)
				}

			case CmdIsWasm:
				if wasm, err := isWasmFile(cmd.path); err != nil {
					m.printErrMsg(t, fmt.Sprintf("can't exec %s: %v", cmd.path, err))
					return
				} else if !wasm {
					m.printErrMsg(t, fmt.Sprintf("can't exec %s: non-WASM file", cmd.path))
					return
				}

				exeCmd = exec.Command(cmd.path, args[1:]...)
			}

			exeCmd.Env = unpackMap2(exeEnv) // TODO: avoid repacking env map since exec.Start just uses a map[string]string anyway
			exeCmd.Stdin = m.stdio.ReadCloser
			exeCmd.Stdout = t
			exeCmd.Stderr = t

			if _, err := exeCmd.Run(); err != nil {
				m.printErrMsg(t, err.Error())
			}
		} else {
			m.printErrMsg(t, "command or executable not found")
		}
	}

	var scanner *bufio.Scanner = nil
	if m.scriptMode {
		scanner = bufio.NewScanner(m.stdio.ReadCloser)
	}

	for {
		if !m.scriptMode {
			wd, err := os.Getwd()
			if err != nil {
				panic(err)
			}
			t.SetPrompt(fmt.Sprintf("%s ▶ ", wd))
		}

		for _, kvp := range os.Environ() {
			parts := strings.SplitN(kvp, "=", 2)
			shellEnv[parts[0]] = parts[1]
		}

		var line string
		if m.scriptMode {
			m.lineNum++

			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					log.Fatal("fatal:", err)
				}
			}
			line = scanner.Text()
			if line == "" {
				return nil
			}
		} else {
			var err error
			line, err = t.ReadLine()
			if err != nil {
				log.Fatal("fatal:", err)
			}
			if line == "" {
				continue
			}
		}

		// this needs to be rethought, but is left here
		// to be re-implemented somewhere else later
		// --
		// if strings.HasPrefix(line, "@") && !js.Global().Get("wanix").Get("collab").IsUndefined() {
		// 	parts := strings.SplitN(line, " ", 2)
		// 	js.Global().Get("wanix").Get("jazzfs").Get("sendMessage").Invoke(strings.TrimPrefix(parts[0], "@"), parts[1])
		// 	continue
		// }

		args, err := shlex.Split(line, true)
		if err != nil {
			m.printErrMsg(t, fmt.Sprintln("shell parsing error:", err))
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
			m.printErrMsg(t, "missing command or executable")
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

func (m *Shell) ifScriptPrintErr(w io.Writer) {
	if m.scriptMode {
		io.WriteString(w, fmt.Sprintf("script error on line %d", m.lineNum))
	}
}

func (m *Shell) printErrMsg(w io.Writer, msg string) {
	if m.scriptMode {
		io.WriteString(w, fmt.Sprintf("script error on line %d: %s\n", m.lineNum, msg))
	} else {
		io.WriteString(w, fmt.Sprintf("%s\n", msg))
	}
}

type iorwc struct {
	io.ReadCloser
	io.WriteCloser
}

func (io *iorwc) Close() error {
	return nil
}
