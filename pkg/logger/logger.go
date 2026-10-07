// Package logger builds the structured JSON slog.Logger used across the application.
package logger

import (
	"io"
	"log/slog"
	"strings"
)

// New returns a JSON slog.Logger writing to w at the named level ("debug",
// "info", "warn" or "error"). Unknown levels fall back to info.
func New(w io.Writer, level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl}))
}
