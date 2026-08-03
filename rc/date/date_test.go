package date

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testTime = time.Date(
	2026,
	time.July,
	25,
	10,
	34,
	56,
	0,
	time.FixedZone("PDT", -7*60*60),
)

func TestDefaultFormat(t *testing.T) {
	cmd := newCommand(func() time.Time { return testTime }, testTime.Location())
	var stdout bytes.Buffer
	cmd.SetIO(strings.NewReader(""), &stdout, &bytes.Buffer{})

	if err := cmd.RunContext(context.Background()); err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	if got, want := stdout.String(), "Sat Jul 25 10:34:56 PDT 2026\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestUTC(t *testing.T) {
	cmd := newCommand(func() time.Time { return testTime }, testTime.Location())
	var stdout bytes.Buffer
	cmd.SetIO(strings.NewReader(""), &stdout, &bytes.Buffer{})

	if err := cmd.RunContext(context.Background(), "-u"); err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	if got, want := stdout.String(), "Sat Jul 25 17:34:56 UTC 2026\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestCustomFormat(t *testing.T) {
	cmd := newCommand(func() time.Time { return testTime }, testTime.Location())
	var stdout bytes.Buffer
	cmd.SetIO(strings.NewReader(""), &stdout, &bytes.Buffer{})

	if err := cmd.RunContext(
		context.Background(),
		"+%Y-%m-%dT%H:%M:%S%z week=%V day=%j %%",
	); err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	if got, want := stdout.String(), "2026-07-25T10:34:56-0700 week=30 day=206 %\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestReferenceFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reference")
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	modTime := time.Date(2024, time.February, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}

	cmd := newCommand(func() time.Time { return testTime }, testTime.Location())
	cmd.SetWorkingDir(dir)
	var stdout bytes.Buffer
	cmd.SetIO(strings.NewReader(""), &stdout, &bytes.Buffer{})

	if err := cmd.RunContext(context.Background(), "-u", "-r", "reference", "+%s"); err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	if got, want := stdout.String(), "1706933106\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestSettingClockIsRejected(t *testing.T) {
	cmd := newCommand(func() time.Time { return testTime }, testTime.Location())
	cmd.SetIO(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})

	err := cmd.RunContext(context.Background(), "072510342026")
	if err == nil || !strings.Contains(err.Error(), "setting the system clock is not supported") {
		t.Fatalf("RunContext() error = %v, want unsupported clock-setting error", err)
	}
}
