package httpserver

import (
	"log/slog"
	"net/http"

	"voice/backend/pkg/correlation"
	voicelog "voice/backend/pkg/logging"
	voicemw "voice/backend/pkg/middleware"
)

// NewLogger returns a JSON slog logger for service with LOG_LEVEL from env.
func NewLogger(service string) *slog.Logger {
	return voicelog.NewJSONLogger(voicelog.LevelFromEnv(), slog.String("service", service))
}

// Wrap applies request id generation and structured HTTP access logging.
func Wrap(handler http.Handler, logger *slog.Logger) http.Handler {
	if handler == nil {
		handler = http.NotFoundHandler()
	}
	if logger == nil {
		logger = NewLogger("unknown")
	}
	extra := func(r *http.Request) []slog.Attr {
		return []slog.Attr{slog.String("event", "http_access")}
	}
	h := voicemw.AccessLog(logger, correlation.RequestIDHeader, extra)(handler)
	h = voicemw.RequestID(correlation.GenerateRequestID)(h)
	return h
}
