package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// AccessLog writes one structured line per request (method, path, status, duration, request id).
// If extra is non-nil, its attrs are appended (e.g. route_group, remote_addr).
func AccessLog(logger *slog.Logger, requestIDHeader string, extra func(*http.Request) []slog.Attr) func(http.Handler) http.Handler {
	if requestIDHeader == "" {
		requestIDHeader = defaultRequestIDHeader
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(wrapped, r)
			if logger == nil {
				return
			}
			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", wrapped.status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				slog.String("request_id", r.Header.Get(requestIDHeader)),
			}
			if extra != nil {
				attrs = append(attrs, extra(r)...)
			}
			logger.LogAttrs(r.Context(), slog.LevelInfo, "http_request", attrs...)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
