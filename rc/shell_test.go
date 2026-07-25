package rc

import "testing"

func TestLineAfterInterrupt(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{name: "no interrupt", line: "echo hello", want: "echo hello"},
		{name: "idle interrupt", line: "\x03", want: ""},
		{name: "input after interrupt", line: "discarded\x03echo hello", want: "echo hello"},
		{name: "last interrupt wins", line: "\x03old\x03new", want: "new"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lineAfterInterrupt(tt.line); got != tt.want {
				t.Fatalf("lineAfterInterrupt(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}
