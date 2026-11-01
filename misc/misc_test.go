package misc

import (
	"io"
	"slices"
	"testing"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/misc/shlex"
)

func TestEncodeVectorRoundTrip(t *testing.T) {
	in := []string{"program", "line one\nline two", "", "tail"}
	b, err := EncodeVector(in)
	if err != nil {
		t.Fatal(err)
	}
	if got := DecodeVector(b); !slices.Equal(got, in) {
		t.Fatalf("got %q want %q", got, in)
	}
}

func TestEncodeVectorEmpty(t *testing.T) {
	b, err := EncodeVector(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 0 {
		t.Fatalf("empty vector encoded to %q", b)
	}
	if got := DecodeVector(nil); len(got) != 0 {
		t.Fatalf("empty data decoded to %q", got)
	}
}

func TestEncodeVectorEmptyRecords(t *testing.T) {
	if got := DecodeVector([]byte("\x00")); !slices.Equal(got, []string{""}) {
		t.Fatalf("got %q", got)
	}
	if got := DecodeVector([]byte("a\x00\x00b\x00")); !slices.Equal(got, []string{"a", "", "b"}) {
		t.Fatalf("got %q", got)
	}
}

func TestEncodeVectorRejectsNUL(t *testing.T) {
	if _, err := EncodeVector([]string{"a\x00b"}); err == nil {
		t.Fatal("want error for embedded NUL")
	}
}

func TestVectorFileWriteThenRead(t *testing.T) {
	var got []string
	ff := VectorFile(func() []string { return got }, func(v []string) error { got = v; return nil })
	f, err := ff.Open("env")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Write(f, []byte("A=1\x00B=line one\nline two\x00")); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"A=1", "B=line one\nline two"}) {
		t.Fatalf("got %q", got)
	}
}

func TestVectorFileRejectsNonNUL(t *testing.T) {
	called := false
	ff := VectorFile(func() []string { return nil }, func([]string) error {
		called = true
		return nil
	})
	f, err := ff.Open("env")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Write(f, []byte("A=1\nB=2\n")); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err == nil {
		t.Fatal("want error for non-NUL-terminated data")
	}
	if called {
		t.Fatal("setter called for non-NUL-terminated data")
	}
}

func TestVectorFileReadAndMode(t *testing.T) {
	called := false
	ff := VectorFile([]string{"seed"}, func() []string { return []string{"a", "b c"} }, func([]string) error {
		called = true
		return nil
	})
	f, err := ff.Open("env")
	if err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0755 {
		t.Fatalf("mode %v want 0755", got)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != "a\x00b c\x00" {
		t.Fatalf("read %q want %q", got, "a\x00b c\x00")
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("setter called after read")
	}
}

func TestJoinArgsWhitespaceRoundTrip(t *testing.T) {
	in := []string{"a\rb", "a\u00a0b", "a\vb", "a\"b", "plain"}
	got, err := shlex.Split(JoinArgs(in), true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, in) {
		t.Fatalf("got %q want %q", got, in)
	}
}

func TestJoinArgs(t *testing.T) {
	for _, tc := range []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"a", "", "b c", "it's", "x\ny"}, `a '' 'b c' 'it'"'"'s' 'x
y'`},
	} {
		if got := JoinArgs(tc.in); got != tc.want {
			t.Errorf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}
