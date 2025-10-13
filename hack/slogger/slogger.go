package slogger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
)

type Handler struct {
	slog.Handler
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	var attrs []string
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, fmt.Sprintf("\033[90m%s=\033[0m%v", a.Key, a.Value.Any()))
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

	// if pkgName == "httpfs" {
	// 	return nil
	// }

	fmt.Printf("\033[90m%s\033[0m %s: %s %s \033[90m%s:%d\033[0m\r\n", ts, pkgName, r.Message, strings.Join(attrs, " "), shortFile, line)
	return nil
}

func Use(level slog.Level) {
	slog.SetDefault(
		slog.New(
			&Handler{
				Handler: slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
					Level: level,
				}),
			},
		),
	)
}
