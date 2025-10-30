package slogger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"tractor.dev/wanix/internal/glob"
)

type HandlerOptions struct {
	Level   slog.Level
	Include []string // If non-empty, only attrs matching ANY pattern are logged
	Exclude []string // Attrs matching ANY pattern are excluded
}

type Handler struct {
	slog.Handler
	includePatterns  []*regexp.Regexp
	includeOriginals []string
	excludePatterns  []*regexp.Regexp
	excludeOriginals []string
}

// matchesPatterns checks if the attribute matches any of the given patterns
func (h *Handler) matchesPatterns(key string, value any, patterns []*regexp.Regexp, originals []string) bool {
	if len(patterns) == 0 {
		return false
	}

	// Format value, using <nil> for nil values
	var valueStr string
	isNil := value == nil
	if isNil {
		valueStr = "<nil>"
	} else {
		valueStr = fmt.Sprintf("%v", value)
	}

	// Check both "key=value" and "key" patterns
	fullAttr := fmt.Sprintf("%s=%s", key, valueStr)

	for i, pattern := range patterns {
		original := originals[i]

		// Special case: if value is nil and pattern looks like "key=*" or "*=*",
		// don't match. This allows "err=*" to exclude nil errors.
		if isNil && strings.HasSuffix(original, "=*") {
			// Check if the pattern would match the key part
			keyPattern := strings.TrimSuffix(original, "=*")
			keyRegex := glob.ToRegex(keyPattern)
			if matched, _ := regexp.MatchString(keyRegex, key); matched {
				// This pattern is for this key but value is nil, skip it
				continue
			}
		}

		// Match against "key=value"
		if pattern.MatchString(fullAttr) {
			return true
		}
		// Match against just "key"
		if pattern.MatchString(key) {
			return true
		}
	}
	return false
}

// shouldIncludeRecord determines if a log record should be included based on filters
func (h *Handler) shouldIncludeRecord(r slog.Record) bool {
	// Collect all attributes to check
	var hasIncludeMatch bool
	var hasExcludeMatch bool

	r.Attrs(func(a slog.Attr) bool {
		// Check if this attribute matches any include pattern
		if len(h.includePatterns) > 0 {
			if h.matchesPatterns(a.Key, a.Value.Any(), h.includePatterns, h.includeOriginals) {
				hasIncludeMatch = true
			}
		}

		// Check if this attribute matches any exclude pattern
		if len(h.excludePatterns) > 0 {
			if h.matchesPatterns(a.Key, a.Value.Any(), h.excludePatterns, h.excludeOriginals) {
				hasExcludeMatch = true
			}
		}

		return true // continue iteration
	})

	// If include patterns exist, at least one must match
	if len(h.includePatterns) > 0 && !hasIncludeMatch {
		return false
	}

	// If any exclude pattern matches, exclude the record
	if hasExcludeMatch {
		return false
	}

	return true
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	// Check if this record should be included based on filters
	if !h.shouldIncludeRecord(r) {
		return nil
	}

	var attrs []string
	r.Attrs(func(a slog.Attr) bool {
		// Format value, using <nil> for nil values
		var valueStr string
		if a.Value.Any() == nil {
			valueStr = "<nil>"
		} else {
			valueStr = fmt.Sprintf("%v", a.Value.Any())
		}

		attrs = append(attrs, fmt.Sprintf("\033[90m%s=\033[0m%s", a.Key, valueStr))
		return true
	})
	ts := r.Time.Format("15:04:05.000")
	var file string
	var line int
	if r.PC != 0 {
		// Use runtime.CallersFrames to get file/line from PC
		// (import "runtime" at the top if not already)
		frames := []uintptr{r.PC}
		callersFrames := runtime.CallersFrames(frames)
		frame, _ := callersFrames.Next()
		file = frame.File
		line = frame.Line
	}
	if file == "" {
		file = "???"
	}
	shortFile := file
	pkgName := filepath.Base(filepath.Dir(file))
	if idx := strings.LastIndex(file, "/"); idx != -1 {
		shortFile = file[idx+1:]
	}

	fmt.Printf("\033[90m%s\033[0m %s: %s %s \033[90m%s:%d\033[0m\r\n", ts, pkgName, r.Message, strings.Join(attrs, " "), shortFile, line)
	return nil
}

// compilePatterns converts glob patterns to compiled regexes
func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		regexStr := glob.ToRegex(pattern)
		re, err := regexp.Compile(regexStr)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}

func Use(level slog.Level) {
	slog.SetDefault(New(level))
}

func UseWithOptions(opts HandlerOptions) {
	slog.SetDefault(NewWithOptions(opts))
}

func New(level slog.Level) *slog.Logger {
	return NewWithOptions(HandlerOptions{
		Level: level,
	})
}

func NewWithOptions(opts HandlerOptions) *slog.Logger {
	// Compile include patterns
	includePatterns, err := compilePatterns(opts.Include)
	if err != nil {
		panic(fmt.Sprintf("slogger: failed to compile include filters: %v", err))
	}

	// Compile exclude patterns
	excludePatterns, err := compilePatterns(opts.Exclude)
	if err != nil {
		panic(fmt.Sprintf("slogger: failed to compile exclude filters: %v", err))
	}

	return slog.New(
		&Handler{
			Handler: slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
				Level: opts.Level,
			}),
			includePatterns:  includePatterns,
			includeOriginals: opts.Include,
			excludePatterns:  excludePatterns,
			excludeOriginals: opts.Exclude,
		},
	)
}
