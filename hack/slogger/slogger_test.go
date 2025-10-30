package slogger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// captureHandler wraps our handler to capture log output for testing
type captureHandler struct {
	*Handler
	buffer *bytes.Buffer
	logged bool
}

func (h *captureHandler) Handle(ctx context.Context, r slog.Record) error {
	// Check if record should be included
	if !h.Handler.shouldIncludeRecord(r) {
		h.logged = false
		return nil
	}

	h.logged = true

	// Capture attributes for testing
	var attrs []string
	r.Attrs(func(a slog.Attr) bool {
		var valueStr string
		if a.Value.Any() == nil {
			valueStr = "<nil>"
		} else {
			valueStr = a.Value.String()
		}
		attrs = append(attrs, a.Key+"="+valueStr)
		return true
	})
	h.buffer.WriteString(strings.Join(attrs, " "))
	return nil
}

func TestIncludeFilters(t *testing.T) {
	tests := []struct {
		name        string
		filters     []string
		attrs       map[string]any
		shouldLog   bool
		description string
	}{
		{
			name:        "include err=* logs record with non-nil error",
			filters:     []string{"err=*"},
			attrs:       map[string]any{"err": "timeout", "user": "john"},
			shouldLog:   true,
			description: "has err with non-nil value",
		},
		{
			name:        "include err=* excludes record with nil error",
			filters:     []string{"err=*"},
			attrs:       map[string]any{"err": nil, "user": "john"},
			shouldLog:   false,
			description: "err is nil",
		},
		{
			name:        "include err=* excludes record without error",
			filters:     []string{"err=*"},
			attrs:       map[string]any{"user": "john"},
			shouldLog:   false,
			description: "no err attribute",
		},
		{
			name:        "include err logs record with any err value",
			filters:     []string{"err"},
			attrs:       map[string]any{"err": nil, "user": "john"},
			shouldLog:   true,
			description: "has err attribute (even if nil)",
		},
		{
			name:        "include db_* logs record with matching prefix",
			filters:     []string{"db_*"},
			attrs:       map[string]any{"db_name": "users", "user": "john"},
			shouldLog:   true,
			description: "has db_name attribute",
		},
		{
			name:        "include db_* excludes record without match",
			filters:     []string{"db_*"},
			attrs:       map[string]any{"user": "john"},
			shouldLog:   false,
			description: "no db_* attributes",
		},
		{
			name:        "multiple include filters (OR logic)",
			filters:     []string{"err=*", "warn=*"},
			attrs:       map[string]any{"err": "failed", "user": "john"},
			shouldLog:   true,
			description: "matches err=*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := &bytes.Buffer{}
			handler := NewWithOptions(HandlerOptions{
				Level:   slog.LevelDebug,
				Include: tt.filters,
			}).Handler().(*Handler)

			captureH := &captureHandler{
				Handler: handler,
				buffer:  buffer,
			}

			// Create record with test attributes
			record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
			for k, v := range tt.attrs {
				record.AddAttrs(slog.Any(k, v))
			}

			captureH.Handle(context.Background(), record)

			if tt.shouldLog && !captureH.logged {
				t.Errorf("%s: expected record to be logged, but it wasn't", tt.description)
			}
			if !tt.shouldLog && captureH.logged {
				t.Errorf("%s: expected record to be filtered out, but it was logged: %q", tt.description, buffer.String())
			}

			// If it should log, verify all attributes are present
			if tt.shouldLog && captureH.logged {
				output := buffer.String()
				for k := range tt.attrs {
					if !strings.Contains(output, k+"=") {
						t.Errorf("expected all attributes in output, missing %q: %q", k, output)
					}
				}
			}
		})
	}
}

func TestExcludeFilters(t *testing.T) {
	tests := []struct {
		name        string
		filters     []string
		attrs       map[string]any
		shouldLog   bool
		description string
	}{
		{
			name:        "exclude debug_* filters record with debug attribute",
			filters:     []string{"debug_*"},
			attrs:       map[string]any{"debug_flag": true, "user": "john"},
			shouldLog:   false,
			description: "has debug_flag attribute",
		},
		{
			name:        "exclude debug_* allows record without debug attribute",
			filters:     []string{"debug_*"},
			attrs:       map[string]any{"user": "john"},
			shouldLog:   true,
			description: "no debug_* attributes",
		},
		{
			name:        "exclude multiple patterns filters matching record",
			filters:     []string{"debug_*", "trace_*"},
			attrs:       map[string]any{"trace_id": "123", "user": "john"},
			shouldLog:   false,
			description: "has trace_id attribute",
		},
		{
			name:        "exclude specific value pattern filters exact match",
			filters:     []string{"level=debug"},
			attrs:       map[string]any{"level": "debug", "user": "john"},
			shouldLog:   false,
			description: "level=debug matches",
		},
		{
			name:        "exclude specific value pattern allows different value",
			filters:     []string{"level=debug"},
			attrs:       map[string]any{"level": "info", "user": "john"},
			shouldLog:   true,
			description: "level=info doesn't match level=debug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := &bytes.Buffer{}
			handler := NewWithOptions(HandlerOptions{
				Level:   slog.LevelDebug,
				Exclude: tt.filters,
			}).Handler().(*Handler)

			captureH := &captureHandler{
				Handler: handler,
				buffer:  buffer,
			}

			record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
			for k, v := range tt.attrs {
				record.AddAttrs(slog.Any(k, v))
			}

			captureH.Handle(context.Background(), record)

			if tt.shouldLog && !captureH.logged {
				t.Errorf("%s: expected record to be logged, but it wasn't", tt.description)
			}
			if !tt.shouldLog && captureH.logged {
				t.Errorf("%s: expected record to be filtered out, but it was logged: %q", tt.description, buffer.String())
			}
		})
	}
}

func TestCombinedFilters(t *testing.T) {
	tests := []struct {
		name        string
		include     []string
		exclude     []string
		attrs       map[string]any
		shouldLog   bool
		description string
	}{
		{
			name:        "include db_* but exclude *_trace - matches db_name",
			include:     []string{"db_*"},
			exclude:     []string{"*_trace"},
			attrs:       map[string]any{"db_name": "users", "user": "admin"},
			shouldLog:   true,
			description: "has db_name (matches include), no trace (no exclude match)",
		},
		{
			name:        "include db_* but exclude *_trace - has db_trace",
			include:     []string{"db_*"},
			exclude:     []string{"*_trace"},
			attrs:       map[string]any{"db_name": "users", "db_trace": "trace123"},
			shouldLog:   false,
			description: "has db_trace (matches exclude filter)",
		},
		{
			name:        "include db_* but exclude *_trace - no db attributes",
			include:     []string{"db_*"},
			exclude:     []string{"*_trace"},
			attrs:       map[string]any{"user": "admin"},
			shouldLog:   false,
			description: "no db_* attributes (doesn't match include)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := &bytes.Buffer{}
			handler := NewWithOptions(HandlerOptions{
				Level:   slog.LevelDebug,
				Include: tt.include,
				Exclude: tt.exclude,
			}).Handler().(*Handler)

			captureH := &captureHandler{
				Handler: handler,
				buffer:  buffer,
			}

			record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
			for k, v := range tt.attrs {
				record.AddAttrs(slog.Any(k, v))
			}

			captureH.Handle(context.Background(), record)

			if tt.shouldLog && !captureH.logged {
				t.Errorf("%s: expected record to be logged, but it wasn't", tt.description)
			}
			if !tt.shouldLog && captureH.logged {
				t.Errorf("%s: expected record to be filtered out, but it was logged: %q", tt.description, buffer.String())
			}
		})
	}
}

func TestNilValueFormatting(t *testing.T) {
	buffer := &bytes.Buffer{}
	handler := NewWithOptions(HandlerOptions{
		Level: slog.LevelDebug,
	}).Handler().(*Handler)

	captureH := &captureHandler{
		Handler: handler,
		buffer:  buffer,
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	record.AddAttrs(slog.Any("err", nil))

	captureH.Handle(context.Background(), record)

	output := buffer.String()
	if !strings.Contains(output, "err=<nil>") {
		t.Errorf("expected nil to be formatted as <nil>, got: %q", output)
	}
}
