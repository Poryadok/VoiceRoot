package logging

import (
	"log/slog"
	"testing"
)

func TestParseLevel(t *testing.T) {
	if ParseLevel("") != slog.LevelInfo {
		t.Fatal()
	}
	if ParseLevel("debug") != slog.LevelDebug {
		t.Fatal()
	}
	if ParseLevel("WARN") != slog.LevelWarn {
		t.Fatal()
	}
	if ParseLevel("error") != slog.LevelError {
		t.Fatal()
	}
	if ParseLevel("unknown") != slog.LevelInfo {
		t.Fatal()
	}
}

func TestLevelFromEnv(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")
	if LevelFromEnv() != slog.LevelDebug {
		t.Fatal()
	}
}
