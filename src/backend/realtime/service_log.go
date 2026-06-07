package main

import (
	"log/slog"

	"voice/backend/pkg/httpserver"
)

var svcLogger = slog.Default()

func initServiceLogger(service string) *slog.Logger {
	svcLogger = httpserver.NewLogger(service)
	return svcLogger
}
