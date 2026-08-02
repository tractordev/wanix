package watch

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRunsImmediatelyAndRepeats(t *testing.T) {
	var calls [][]string
	var waits []time.Duration
	waitCount := 0
	var stdout bytes.Buffer
	cmd := newCommand(
		func(_ context.Context, args []string, commandStdin io.Reader, commandStdout, commandStderr io.Writer) error {
			calls = append(calls, append([]string(nil), args...))
			input, err := io.ReadAll(commandStdin)
			if err != nil {
				return err
			}
			if len(input) != 0 {
				t.Fatalf("command stdin = %q, want EOF", input)
			}
			if _, err := io.WriteString(commandStdout, "stdout\n"); err != nil {
				return err
			}
			if _, err := io.WriteString(commandStderr, "stderr\n"); err != nil {
				return err
			}
			return nil
		},
		func(_ context.Context, interval time.Duration) bool {
			waits = append(waits, interval)
			waitCount++
			return waitCount == 1
		},
	)
	cmd.SetIO(strings.NewReader("terminal input"), &stdout, &bytes.Buffer{})

	if err := cmd.RunContext(context.Background(), "-n", "5", "ls", "-l"); err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	wantCalls := [][]string{{"ls", "-l"}, {"ls", "-l"}}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
	wantWaits := []time.Duration{5 * time.Second, 5 * time.Second}
	if !reflect.DeepEqual(waits, wantWaits) {
		t.Fatalf("waits = %#v, want %#v", waits, wantWaits)
	}
	if got := stdout.String(); strings.Count(got, "\033[0;0H\033[J") != 2 {
		t.Fatalf("clear sequence count = %d, want 2; output %q", strings.Count(got, "\033[0;0H\033[J"), got)
	}
	if got := stdout.String(); strings.Count(got, "Every 5 : [ls -l]") != 2 {
		t.Fatalf("header count = %d, want 2; output %q", strings.Count(got, "Every 5 : [ls -l]"), got)
	}
	if got := stdout.String(); strings.Count(got, "stdout\nstderr\n") != 2 {
		t.Fatalf("command output count = %d, want 2; output %q", strings.Count(got, "stdout\nstderr\n"), got)
	}
}

func TestNoTitle(t *testing.T) {
	cmd := newCommand(
		func(_ context.Context, _ []string, _ io.Reader, _, _ io.Writer) error { return nil },
		func(_ context.Context, _ time.Duration) bool { return false },
	)
	var stdout bytes.Buffer
	cmd.SetIO(strings.NewReader(""), &stdout, &bytes.Buffer{})

	if err := cmd.RunContext(context.Background(), "-t", "ls"); err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	if got := stdout.String(); got != "\033[0;0H\033[J" {
		t.Fatalf("output = %q, want only terminal clear sequence", got)
	}
}

func TestNoCommandPrintsUsage(t *testing.T) {
	called := false
	cmd := newCommand(
		func(_ context.Context, _ []string, _ io.Reader, _, _ io.Writer) error {
			called = true
			return nil
		},
		func(_ context.Context, _ time.Duration) bool { return false },
	)
	var stderr bytes.Buffer
	cmd.SetIO(strings.NewReader(""), &bytes.Buffer{}, &stderr)

	if err := cmd.RunContext(context.Background()); err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	if called {
		t.Fatal("dispatcher was called without a command")
	}
	if got := stderr.String(); !strings.Contains(got, "Usage: watch") {
		t.Fatalf("stderr = %q, want usage", got)
	}
}

func TestCancellationStopsBeforeWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	waited := false
	cmd := newCommand(
		func(_ context.Context, _ []string, _ io.Reader, _, _ io.Writer) error {
			calls++
			cancel()
			return context.Canceled
		},
		func(_ context.Context, _ time.Duration) bool {
			waited = true
			return true
		},
	)
	cmd.SetIO(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})

	if err := cmd.RunContext(ctx, "ls"); err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	if calls != 1 {
		t.Fatalf("dispatcher calls = %d, want 1", calls)
	}
	if waited {
		t.Fatal("wait called after context cancellation")
	}
}

func TestCtrlCStopsWatch(t *testing.T) {
	stdin, input := io.Pipe()
	defer stdin.Close()
	defer input.Close()

	executed := make(chan struct{})
	cmd := newCommand(
		func(_ context.Context, _ []string, _ io.Reader, _, _ io.Writer) error {
			select {
			case <-executed:
			default:
				close(executed)
			}
			return nil
		},
		waitForInterval,
	)
	cmd.SetIO(stdin, &bytes.Buffer{}, &bytes.Buffer{})

	done := make(chan error, 1)
	go func() {
		done <- cmd.RunContext(context.Background(), "-n", "60", "ls")
	}()

	select {
	case <-executed:
	case <-time.After(time.Second):
		t.Fatal("watch did not execute its command")
	}

	if _, err := input.Write([]byte{'\x03', '\n'}); err != nil {
		t.Fatalf("writing Ctrl-C: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunContext() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("watch did not stop after Ctrl-C")
	}
}

func TestCommandErrorsDoNotStopWatching(t *testing.T) {
	calls := 0
	waits := 0
	cmd := newCommand(
		func(_ context.Context, _ []string, _ io.Reader, _, _ io.Writer) error {
			calls++
			return errors.New("command failed")
		},
		func(_ context.Context, _ time.Duration) bool {
			waits++
			return waits == 1
		},
	)
	cmd.SetIO(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})

	if err := cmd.RunContext(context.Background(), "false"); err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	if calls != 2 {
		t.Fatalf("dispatcher calls = %d, want 2", calls)
	}
}

func TestNonPositiveIntervalUsesDefault(t *testing.T) {
	var got time.Duration
	cmd := newCommand(
		func(_ context.Context, _ []string, _ io.Reader, _, _ io.Writer) error { return nil },
		func(_ context.Context, interval time.Duration) bool {
			got = interval
			return false
		},
	)
	cmd.SetIO(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})

	if err := cmd.RunContext(context.Background(), "-n", "0", "ls"); err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	if want := 2 * time.Second; got != want {
		t.Fatalf("interval = %v, want %v", got, want)
	}
}

func TestDifferencesAreHighlighted(t *testing.T) {
	outputs := []string{"value: 10\n", "value: 12\n"}
	call := 0
	wait := 0
	cmd := newCommand(
		func(_ context.Context, _ []string, _ io.Reader, stdout, _ io.Writer) error {
			_, err := io.WriteString(stdout, outputs[call])
			call++
			return err
		},
		func(_ context.Context, _ time.Duration) bool {
			wait++
			return wait == 1
		},
	)
	var stdout bytes.Buffer
	cmd.SetIO(strings.NewReader(""), &stdout, &bytes.Buffer{})

	if err := cmd.RunContext(context.Background(), "-d", "-n", "1", "counter"); err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "value: 10\n") {
		t.Fatalf("first output was unexpectedly highlighted or missing: %q", got)
	}
	if !strings.Contains(got, "value: 1\x1b[7m2\x1b[0m\n") {
		t.Fatalf("second output does not highlight the changed character: %q", got)
	}
}

func TestHighlightChanges(t *testing.T) {
	tests := []struct {
		name     string
		previous string
		current  string
		want     string
	}{
		{name: "deleted characters", previous: "abc\n", current: "a\n", want: "a\x1b[7m  \x1b[0m\n"},
		{name: "Unicode runes", previous: "café\n", current: "cafe\n", want: "caf\x1b[7me\x1b[0m\n"},
		{name: "ANSI output", previous: "plain\n", current: "\x1b[32mgreen\x1b[0m\n", want: "\x1b[32mgreen\x1b[0m\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := highlightChanges(tt.previous, tt.current); got != tt.want {
				t.Fatalf("highlightChanges() = %q, want %q", got, tt.want)
			}
		})
	}
}
