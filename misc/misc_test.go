package misc

import (
	"slices"
	"testing"

	"tractor.dev/wanix/fs"
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
