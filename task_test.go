package wanix

import (
	"slices"
	"testing"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/misc/shlex"
)

func newTestRoot(t *testing.T) *Task {
	t.Helper()
	root, err := NewRootWithTasks(NewTaskFS())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func writeTaskFile(t *testing.T, root *Task, name, data string) {
	t.Helper()
	f, err := root.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Write(f, []byte(data)); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTaskVectorFilesWrite(t *testing.T) {
	root := newTestRoot(t)
	writeTaskFile(t, root, "env", "A=1\x00B=line one\nline two\x00")
	want := []string{"A=1", "B=line one\nline two"}
	if got := root.Env(); !slices.Equal(got, want) {
		t.Fatalf("env got %q want %q", got, want)
	}
}

func TestTaskArgvVector(t *testing.T) {
	root := newTestRoot(t)
	writeTaskFile(t, root, "argv", "prog\x00line one\nline two\x00\x00tail\x00")
	want := []string{"prog", "line one\nline two", "", "tail"}
	if got := root.Args(); !slices.Equal(got, want) {
		t.Fatalf("args got %q want %q", got, want)
	}
}

func TestTaskArgvUpdatesCmd(t *testing.T) {
	root := newTestRoot(t)
	want := []string{"prog", "line one\nline two", "", "a b", "it's", `a\b`}
	writeTaskFile(t, root, "argv", "prog\x00line one\nline two\x00\x00a b\x00it's\x00a\\b\x00")
	got, err := shlex.Split(root.Cmd(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("cmd %q split got %q want %q", root.Cmd(), got, want)
	}
}

func TestTaskCmdKeepsQuotedEmpty(t *testing.T) {
	root := newTestRoot(t)
	writeTaskFile(t, root, "cmd", "prog '' 'x y'")
	want := []string{"prog", "", "x y"}
	if got := root.Args(); !slices.Equal(got, want) {
		t.Fatalf("args got %q want %q", got, want)
	}
}

func TestTaskCmdRejectsNUL(t *testing.T) {
	root := newTestRoot(t)
	f, err := root.Open("cmd")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Write(f, []byte("a\x00b")); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err == nil {
		t.Fatal("want error for embedded NUL in cmd")
	}
	if got := root.Args(); len(got) != 0 {
		t.Fatalf("args got %q after rejected cmd", got)
	}
}

func TestTaskEnvRejectsNonNUL(t *testing.T) {
	root := newTestRoot(t)
	writeTaskFile(t, root, "env", "A=1\x00")
	f, err := root.Open("env")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Write(f, []byte("A=1\nB=2\n")); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err == nil {
		t.Fatal("want error for non-NUL-terminated env")
	}
	if got := root.Env(); !slices.Equal(got, []string{"A=1"}) {
		t.Fatalf("env got %q want [\"A=1\"]", got)
	}
}

func TestTaskEnvVector(t *testing.T) {
	root := newTestRoot(t)
	writeTaskFile(t, root, "env", "A=1\x00A=2\x00")
	want := []string{"A=1", "A=2"}
	if got := root.Env(); !slices.Equal(got, want) {
		t.Fatalf("env got %q want %q", got, want)
	}
}
