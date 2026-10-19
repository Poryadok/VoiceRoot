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

func TestAccessLogRedactsSpaceInviteCodesWithoutChangingDispatch(t *testing.T) {
	for _, suffix := range []string{"", "/join", "/join-attempts", "/malformed/extra"} {
		t.Run(suffix, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil))
			path := "/api/v1/invites/secret-invite-code" + suffix
			handler := AccessLog(logger, "X-Request-Id", nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != path {
					t.Fatalf("log redaction changed dispatcher path: %q", r.URL.Path)
				}
				w.WriteHeader(http.StatusOK)
			}))
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, path, nil))
			if strings.Contains(output.String(), "secret-invite-code") {
				t.Fatal("invite code leaked into access log")
			}
			if !strings.Contains(output.String(), "/api/v1/invites/:code") {
				t.Fatal("safe invite route label missing")
			}
		})
	}
}
