package logging

import (
	"log/slog"
	"os"
	"strings"
)

// LevelFromEnv parses LOG_LEVEL (debug, info, warn, error); unknown values default to info.
func LevelFromEnv() slog.Level {
	return ParseLevel(os.Getenv("LOG_LEVEL"))
}

// ParseLevel maps common names to slog levels; empty defaults to info.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "":
		fallthrough
	case "info":
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

// NewJSONLogger returns a JSON slog.Logger to stdout with optional static attrs (e.g. service name).
func NewJSONLogger(level slog.Level, attrs ...slog.Attr) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	if len(attrs) == 0 {
		return slog.New(h)
	}
	args := make([]any, 0, len(attrs)*2)
	for _, a := range attrs {
		args = append(args, a.Key, a.Value.Any())
	}
	return slog.New(h).With(args...)
}
