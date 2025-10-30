package glob

import (
	"regexp"
	"testing"
)

func TestGlobToRegex(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		input   string
		want    bool
	}{
		// Basic wildcards
		{
			name:    "star matches filename",
			pattern: "*.txt",
			input:   "file.txt",
			want:    true,
		},
		{
			name:    "star doesn't match different extension",
			pattern: "*.txt",
			input:   "file.md",
			want:    false,
		},
		{
			name:    "star matches anything without slash",
			pattern: "*",
			input:   "anything",
			want:    true,
		},
		{
			name:    "star doesn't match path separator",
			pattern: "*",
			input:   "path/to/file",
			want:    false,
		},

		// Double star
		{
			name:    "double star matches path",
			pattern: "**",
			input:   "path/to/file",
			want:    true,
		},
		{
			name:    "double star prefix matches deep path",
			pattern: "**/foo",
			input:   "bar/baz/foo",
			want:    true,
		},
		{
			name:    "double star prefix matches immediate",
			pattern: "**/foo",
			input:   "foo",
			want:    true,
		},
		{
			name:    "double star in middle matches nested file",
			pattern: "**/*.txt",
			input:   "dir/subdir/file.txt",
			want:    true,
		},
		{
			name:    "double star between paths matches multiple dirs",
			pattern: "a/**/b",
			input:   "a/x/y/z/b",
			want:    true,
		},
		{
			name:    "double star between paths matches direct",
			pattern: "a/**/b",
			input:   "a/b",
			want:    true,
		},

		// Question mark
		{
			name:    "question mark matches single char",
			pattern: "?.txt",
			input:   "a.txt",
			want:    true,
		},
		{
			name:    "question mark doesn't match multiple chars",
			pattern: "?.txt",
			input:   "ab.txt",
			want:    false,
		},

		// Character classes
		{
			name:    "char class matches included char",
			pattern: "[abc].txt",
			input:   "a.txt",
			want:    true,
		},
		{
			name:    "char class doesn't match excluded char",
			pattern: "[abc].txt",
			input:   "d.txt",
			want:    false,
		},
		{
			name:    "negated char class matches excluded char",
			pattern: "[!abc].txt",
			input:   "d.txt",
			want:    true,
		},
		{
			name:    "negated char class doesn't match included char",
			pattern: "[!abc].txt",
			input:   "a.txt",
			want:    false,
		},
		{
			name:    "char range matches char in range",
			pattern: "[a-z].txt",
			input:   "m.txt",
			want:    true,
		},

		// Brace expansion
		{
			name:    "brace expansion matches first option",
			pattern: "*.{jpg,png}",
			input:   "image.jpg",
			want:    true,
		},
		{
			name:    "brace expansion matches second option",
			pattern: "*.{jpg,png}",
			input:   "image.png",
			want:    true,
		},
		{
			name:    "brace expansion doesn't match non-option",
			pattern: "*.{jpg,png}",
			input:   "image.gif",
			want:    false,
		},
		{
			name:    "brace expansion in path",
			pattern: "{foo,bar}/*.txt",
			input:   "foo/file.txt",
			want:    true,
		},
		{
			name:    "brace expansion in path doesn't match other dirs",
			pattern: "{foo,bar}/*.txt",
			input:   "baz/file.txt",
			want:    false,
		},

		// Combined patterns
		{
			name:    "star slash matches one level",
			pattern: "*/foo",
			input:   "bar/foo",
			want:    true,
		},
		{
			name:    "star slash doesn't match nested",
			pattern: "*/foo",
			input:   "bar/baz/foo",
			want:    false,
		},
		{
			name:    "complex pattern with double star and star",
			pattern: "**/foo/*.txt",
			input:   "a/b/foo/file.txt",
			want:    true,
		},
		{
			name:    "complex pattern with double star and braces go file",
			pattern: "src/**/*.{go,txt}",
			input:   "src/pkg/main.go",
			want:    true,
		},
		{
			name:    "complex pattern with double star and braces txt file",
			pattern: "src/**/*.{go,txt}",
			input:   "src/a/b/c/test.txt",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regex := ToRegex(tt.pattern)
			matched, err := regexp.MatchString(regex, tt.input)
			if err != nil {
				t.Fatalf("Failed to compile regex for pattern %q: %v", tt.pattern, err)
			}

			if matched != tt.want {
				t.Errorf("Match(%q, %q) = %v, want %v\nGenerated regex: %s",
					tt.pattern, tt.input, matched, tt.want, regex)
			}
		})
	}
}

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		input   string
		want    bool
	}{
		{
			name:    "simple star pattern",
			pattern: "*.go",
			input:   "main.go",
			want:    true,
		},
		{
			name:    "double star with extension",
			pattern: "**/*.go",
			input:   "cmd/server/main.go",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Match(tt.pattern, tt.input)
			if err != nil {
				t.Fatalf("Match() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Match(%q, %q) = %v, want %v",
					tt.pattern, tt.input, got, tt.want)
			}
		})
	}
}

// Benchmark to compare regex compilation and matching
func BenchmarkGlobToRegex(b *testing.B) {
	pattern := "src/**/*.{go,txt,md}"
	for i := 0; i < b.N; i++ {
		_ = ToRegex(pattern)
	}
}

func BenchmarkGlobMatch(b *testing.B) {
	pattern := "src/**/*.{go,txt,md}"
	input := "src/pkg/util/helper.go"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Match(pattern, input)
	}
}

func BenchmarkGlobMatchPrecompiled(b *testing.B) {
	pattern := "src/**/*.{go,txt,md}"
	input := "src/pkg/util/helper.go"
	regex := ToRegex(pattern)
	compiled, err := regexp.Compile(regex)
	if err != nil {
		b.Fatalf("Failed to compile regex: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = compiled.MatchString(input)
	}
}

// Test edge cases
func TestGlobToRegexEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		input   string
		want    bool
	}{
		{
			name:    "empty pattern matches empty string",
			pattern: "",
			input:   "",
			want:    true,
		},
		{
			name:    "escaped star is literal",
			pattern: "\\*.txt",
			input:   "*.txt",
			want:    true,
		},
		{
			name:    "escaped question mark is literal",
			pattern: "file\\?.txt",
			input:   "file?.txt",
			want:    true,
		},
		{
			name:    "unclosed bracket treated as literal",
			pattern: "[abc.txt",
			input:   "[abc.txt",
			want:    true,
		},
		{
			name:    "unclosed brace treated as literal",
			pattern: "{abc.txt",
			input:   "{abc.txt",
			want:    true,
		},
		{
			name:    "nested braces",
			pattern: "{a,{b,c}}.txt",
			input:   "b.txt",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regex := ToRegex(tt.pattern)
			matched, err := regexp.MatchString(regex, tt.input)
			if err != nil {
				t.Fatalf("Failed to compile regex for pattern %q: %v", tt.pattern, err)
			}

			if matched != tt.want {
				t.Errorf("Match(%q, %q) = %v, want %v\nGenerated regex: %s",
					tt.pattern, tt.input, matched, tt.want, regex)
			}
		})
	}
}
