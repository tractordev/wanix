package slogger_test

import (
	"log/slog"

	"tractor.dev/wanix/hack/slogger"
)

func ExampleNewWithOptions_includeErrors() {
	// Create a logger that only logs entries with non-nil error attributes
	logger := slogger.NewWithOptions(slogger.HandlerOptions{
		Level:   slog.LevelDebug,
		Include: []string{"err=*"}, // Only log entries with non-nil errors
	})

	// This ENTIRE ENTRY will be logged (has non-nil err)
	logger.Info("operation failed", "err", "connection timeout", "user", "john")

	// This entry will NOT be logged (err is nil, doesn't match err=*)
	logger.Info("operation succeeded", "err", nil, "user", "john")

	// This entry will NOT be logged (no err attribute)
	logger.Info("processing", "user", "john")
}

func ExampleNewWithOptions_excludeDebug() {
	// Create a logger that excludes entries with debug/trace attributes
	logger := slogger.NewWithOptions(slogger.HandlerOptions{
		Level:   slog.LevelDebug,
		Exclude: []string{"debug_*", "trace_*"},
	})

	// This entry will NOT be logged (has debug_flag attribute)
	logger.Info("processing", "user", "john", "debug_flag", true, "trace_id", "abc123")

	// This entry WILL be logged (no debug_* or trace_* attributes)
	logger.Info("completed", "user", "john", "result", "success")
}

func ExampleNewWithOptions_keyOnlyPattern() {
	// Create a logger that logs entries with "err" key (regardless of value)
	logger := slogger.NewWithOptions(slogger.HandlerOptions{
		Level:   slog.LevelDebug,
		Include: []string{"err"}, // Match key only
	})

	// This ENTIRE ENTRY will be logged (has err key, even though nil)
	logger.Info("operation", "err", nil, "user", "john")

	// This ENTIRE ENTRY will be logged (has err key)
	logger.Info("operation", "err", "timeout", "user", "john")

	// This entry will NOT be logged (no err key)
	logger.Info("operation", "user", "john", "status", "ok")
}

func ExampleNewWithOptions_combined() {
	// Log database operations, but exclude entries with trace info
	logger := slogger.NewWithOptions(slogger.HandlerOptions{
		Level:   slog.LevelDebug,
		Include: []string{"db_*", "query=*"},
		Exclude: []string{"*_trace"},
	})

	// This ENTIRE ENTRY will be logged (has db_name, no trace)
	logger.Info("query", "db_name", "users", "query", "SELECT", "user", "admin")

	// This entry will NOT be logged (has db_trace - excluded)
	logger.Info("query", "db_name", "users", "db_trace", "trace123")

	// This entry will NOT be logged (no db_* or query=* - doesn't match include)
	logger.Info("other", "user", "admin", "action", "login")
}

func ExampleUseWithOptions() {
	// Set global logger to only log entries with errors
	slogger.UseWithOptions(slogger.HandlerOptions{
		Level:   slog.LevelDebug,
		Include: []string{"err=*"},
	})

	// Now the default slog logger uses the filtered handler

	// This entry will be logged (has non-nil error)
	slog.Info("error occurred", "err", "connection failed")

	// This entry will NOT be logged (no error attribute)
	slog.Info("success", "user", "admin")
}
