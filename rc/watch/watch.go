// Copyright 2015-2021 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package watch periodically executes a command.
//
// This implementation is adapted from u-root's cmds/exp/watch command. Instead
// of os/exec, it uses an injected dispatcher so it works inside Wanix's
// browser-hosted rc shell.
package watch

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/u-root/u-root/pkg/core"
	"github.com/u-root/u-root/pkg/uroot/unixflag"
)

const defaultInterval = int64(2)

type execFunc func(context.Context, []string, io.Reader, io.Writer, io.Writer) error
type waitFunc func(context.Context, time.Duration) bool

type command struct {
	core.Base
	exec execFunc
	wait waitFunc
}

// New creates a new watch command that invokes commands through exec.
func New(exec func(context.Context, []string, io.Reader, io.Writer, io.Writer) error) core.Command {
	return newCommand(exec, waitForInterval)
}

func newCommand(exec execFunc, wait waitFunc) *command {
	c := &command{
		exec: exec,
		wait: wait,
	}
	c.Init()
	return c
}

func (c *command) Run(args ...string) error {
	return c.RunContext(context.Background(), args...)
}

func (c *command) RunContext(ctx context.Context, args ...string) error {
	var showDifferences, noTitle bool
	var seconds int64

	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	fs.BoolVar(&showDifferences, "d", false, "highlight changes between updates")
	fs.BoolVar(&noTitle, "t", false, "do not print header")
	fs.Int64Var(&seconds, "n", defaultInterval, "loop period in seconds")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: watch [-d] [-n SEC] [-t] PROG [ARGS...]")
		fmt.Fprintln(fs.Output(), "Run PROG periodically")
		fs.PrintDefaults()
	}

	if err := fs.Parse(unixflag.ArgsToGoArgs(args)); err != nil {
		return err
	}
	commandArgs := fs.Args()
	if len(commandArgs) == 0 {
		fs.Usage()
		return nil
	}
	if c.exec == nil {
		return fmt.Errorf("command dispatcher is not configured")
	}

	commandCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go cancelOnInterrupt(c.Stdin, cancel)

	intervalSeconds := seconds
	if intervalSeconds <= 0 {
		intervalSeconds = defaultInterval
	}
	interval := time.Duration(intervalSeconds) * time.Second

	var currentOutput bytes.Buffer
	commandStdin := strings.NewReader("")
	var previousOutput string
	havePreviousOutput := false
	for {
		if commandCtx.Err() != nil {
			return nil
		}

		currentOutput.Reset()
		_ = c.exec(commandCtx, commandArgs, commandStdin, &currentOutput, &currentOutput)
		if commandCtx.Err() != nil {
			return nil
		}

		c.drawHeader(commandArgs, intervalSeconds, noTitle)
		if showDifferences {
			output := currentOutput.String()
			if havePreviousOutput {
				fmt.Fprint(c.Stdout, highlightChanges(previousOutput, output))
			} else {
				fmt.Fprint(c.Stdout, output)
			}
			previousOutput = output
			havePreviousOutput = true
		} else {
			_, _ = currentOutput.WriteTo(c.Stdout)
		}

		if !c.wait(commandCtx, interval) {
			return nil
		}
	}
}

func (c *command) drawHeader(commandArgs []string, intervalSeconds int64, noTitle bool) {
	fmt.Fprint(c.Stdout, "\033[0;0H\033[J")
	if !noTitle {
		fmt.Fprintf(c.Stdout, "Every %d : %v \n\n", intervalSeconds, commandArgs)
	}
}

func highlightChanges(previous, current string) string {
	if strings.ContainsRune(previous, '\x1b') || strings.ContainsRune(current, '\x1b') {
		return current
	}

	previousLines := strings.Split(previous, "\n")
	currentLines := strings.Split(current, "\n")
	lineCount := max(len(previousLines), len(currentLines))
	highlighted := make([]string, lineCount)
	for i := range lineCount {
		var previousLine, currentLine string
		if i < len(previousLines) {
			previousLine = previousLines[i]
		}
		if i < len(currentLines) {
			currentLine = currentLines[i]
		}
		highlighted[i] = highlightLine(previousLine, currentLine)
	}
	return strings.Join(highlighted, "\n")
}

func highlightLine(previous, current string) string {
	previousRunes := []rune(previous)
	currentRunes := []rune(current)
	width := max(len(previousRunes), len(currentRunes))

	var out strings.Builder
	highlighting := false
	for i := range width {
		r := ' '
		if i < len(currentRunes) {
			r = currentRunes[i]
		}
		changed := i >= len(previousRunes) || i >= len(currentRunes) || previousRunes[i] != r
		if changed != highlighting {
			if changed {
				out.WriteString("\x1b[7m")
			} else {
				out.WriteString("\x1b[0m")
			}
			highlighting = changed
		}
		out.WriteRune(r)
	}
	if highlighting {
		out.WriteString("\x1b[0m")
	}
	return out.String()
}

func cancelOnInterrupt(stdin io.Reader, cancel context.CancelFunc) {
	if stdin == nil {
		return
	}

	buf := make([]byte, 64)
	interrupted := false
	for {
		n, err := stdin.Read(buf)
		for _, b := range buf[:n] {
			if b == '\x03' {
				interrupted = true
			}
			if interrupted && b == '\n' {
				cancel()
				return
			}
		}
		if err != nil {
			if interrupted {
				cancel()
			}
			return
		}
	}
}

func waitForInterval(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
