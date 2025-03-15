package fskit

import (
	"reflect"
	"testing"
)

func TestMatchPaths(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		testPath string
		want     []string
	}{
		{
			name:     "empty paths",
			paths:    []string{},
			testPath: "foo/bar",
			want:     nil,
		},
		{
			name:     "single exact match",
			paths:    []string{"foo/bar"},
			testPath: "foo/bar",
			want:     []string{"foo/bar"},
		},
		{
			name:     "prefix match",
			paths:    []string{"foo", "foo/bar"},
			testPath: "foo/bar/baz",
			want:     []string{"foo/bar", "foo"},
		},
		{
			name:     "dot path",
			paths:    []string{"foo", ".", "bar"},
			testPath: "something",
			want:     []string{"."},
		},
		{
			name:     "no matches",
			paths:    []string{"foo", "bar"},
			testPath: "baz/qux",
			want:     nil,
		},
		{
			name:     "longest path first",
			paths:    []string{"web/foo", "web/foo/bar", "bar", "web"},
			testPath: "web/foo/bar/baz",
			want:     []string{"web/foo/bar", "web/foo", "web"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchPaths(tt.paths, tt.testPath)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchPaths() = %v, want %v", got, tt.want)
			}
		})
	}
}
