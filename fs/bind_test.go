package fs

import (
	"strings"
	"reflect"
	"testing"
)

func TestParseBindOptions(t *testing.T) {
	m := ParseBindOptions(BindAfter, BindNS, "foo=bar", "tag")
	want := map[string]string{
		"after": "",
		"type":  "ns",
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

func TestBindTypeOf(t *testing.T) {
	tests := []struct {
		opts []BindOption
		want BindType
	}{
		{nil, "ns"},
		{[]BindOption{BindAfter}, "ns"},
		{[]BindOption{BindNS, BindReplace}, "ns"},
		{[]BindOption{"type=ns"}, "ns"},
	}
	for _, tt := range tests {
		if got := BindTypeOf(tt.opts...); got != tt.want {
			t.Fatalf("BindTypeOf(%v) = %q, want %q", tt.opts, got, tt.want)
		}
	}
}


func TestTrimSpaceSmoke20260716(t *testing.T) {
	if strings.TrimSpace("  x  ") != "x" {
		t.Fatalf("trim space smoke failed")
	}
}
