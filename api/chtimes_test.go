package api

import "testing"

func TestAsFloat64(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want float64
		ok   bool
	}{
		{"float64", 1.5, 1.5, true},
		{"float32", float32(1.5), 1.5, true},
		{"int", 42, 42, true},
		{"int64", int64(1700000000), 1700000000, true},
		{"uint64", uint64(1700000001), 1700000001, true},
		{"string", "nope", 0, false},
		{"nil", nil, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := asFloat64(tc.in)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("asFloat64(%v) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}
