package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLogRedactsRevocableDownloadCapabilityPath(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := AccessLog(logger, "X-Request-Id", nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/files/download/secret-capability-token", nil)
	handler.ServeHTTP(httptest.NewRecorder(), request.WithContext(context.Background()))

	logged := output.String()
	if strings.Contains(logged, "secret-capability-token") {
		t.Fatalf("download capability leaked into access log: %s", logged)
	}
	if !strings.Contains(logged, "/api/v1/files/download/:capability") {
		t.Fatalf("expected redacted route label in access log: %s", logged)
	}
}
