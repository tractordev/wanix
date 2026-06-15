package fs

import (
	"reflect"
	"testing"
)

func TestParseBindOptions(t *testing.T) {
	m := ParseBindOptions(BindAfter, "foo=bar", "tag")
	want := map[string]string{
		"after": "",
		"foo":   "bar",
		"tag":   "",
	}
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("got %v, want %v", m, want)
	}
}

func TestBindPlacement(t *testing.T) {
	tests := []struct {
		opts []BindOption
		want BindOption
	}{
		{nil, BindAfter},
		{[]BindOption{BindReplace, BindAfter}, BindReplace},
		{[]BindOption{"foo=bar", BindBefore}, BindBefore},
	}
	for _, tt := range tests {
		if got := BindPlacement(tt.opts...); got != tt.want {
			t.Fatalf("BindPlacement(%v) = %q, want %q", tt.opts, got, tt.want)
		}
	}
}
