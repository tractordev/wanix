//go:build !js

package rc

import "testing"

func TestEncodeVector(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want string
	}{
		{"empty", nil, ""},
		{"entries", []string{"prog", "line one\nline two", "", "tail"}, "prog\x00line one\nline two\x00\x00tail\x00"},
	} {
		if got := string(encodeVector(tc.in)); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}
