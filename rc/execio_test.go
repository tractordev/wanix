//go:build !js

package rc

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

func TestExecIO(t *testing.T) {
	tests := []struct {
		name             string
		source           string
		wantStdout       string
		wantCaptured     string
		capturedContains string
		wantErr          bool
	}{
		{
			name:       "default output",
			source:     "cat <<'EOF'\nterminal\nEOF",
			wantStdout: "terminal\n",
		},
		{
			name:         "captured input and output",
			source:       "capture cat",
			wantCaptured: "captured input\n",
		},
		{
			name:             "captured env",
			source:           "capture env",
			capturedContains: "EXECIO_TEST=value\n",
		},
		{
			name:             "captured external error",
			source:           "capture command-that-does-not-exist",
			capturedContains: "rc: command-that-does-not-exist:",
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr, captured bytes.Buffer
			runner := newExecIOTestRunner(t, &stdout, &stderr, &captured)
			parser := syntax.NewParser()
			err := runSource(
				context.Background(),
				runner,
				parser,
				"<test>",
				strings.NewReader(tt.source+"\n"),
			)
			if (err != nil) != tt.wantErr {
				t.Fatalf("runSource() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Fatalf("stdout = %q, want %q", got, tt.wantStdout)
			}
			if got := stderr.String(); got != "" {
				t.Fatalf("stderr = %q, want empty", got)
			}
			if got := captured.String(); got != tt.wantCaptured &&
				(tt.capturedContains == "" || !strings.Contains(got, tt.capturedContains)) {
				t.Fatalf("captured output = %q, want %q or containing %q", got, tt.wantCaptured, tt.capturedContains)
			}
		})
	}
}

func newExecIOTestRunner(
	t *testing.T,
	stdout,
	stderr,
	captured *bytes.Buffer,
) *interp.Runner {
	t.Helper()

	captureMiddleware := func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if len(args) > 0 && args[0] == "capture" {
				return next(withExecIO(
					ctx,
					strings.NewReader("captured input\n"),
					captured,
					captured,
				), args[1:])
			}
			return next(ctx, args)
		}
	}

	runner, err := interp.New(
		interp.Dir(t.TempDir()),
		interp.Env(expand.ListEnviron("EXECIO_TEST=value")),
		interp.StdIO(strings.NewReader(""), stdout, stderr),
		interp.ExecHandlers(
			captureMiddleware,
			urootCoreutilsMiddleware(),
			wanixExecMiddleware(),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	return runner
}
