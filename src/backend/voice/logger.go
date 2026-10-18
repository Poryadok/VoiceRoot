package main

import (
	"log/slog"

	"voice/backend/pkg/httpserver"
)

// newServiceLogger initializes the logger before Voice dependencies start.
func newServiceLogger() *slog.Logger {
	logger := httpserver.NewLogger(serviceName)
	// Principal dependency diagnostics use slog's default logger, including
	// background JWKS refreshes that have no request-scoped logger.
	slog.SetDefault(logger)
	return logger
}
