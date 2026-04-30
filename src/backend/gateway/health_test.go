package main

import (
	"net/http"
	"testing"
)

func TestHealth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		wantStatus int
		wantBody   string
	}{
		{name: "GET returns ok", method: http.MethodGet, wantStatus: http.StatusOK, wantBody: "ok"},
		{name: "POST is not accepted as a health check", method: http.MethodPost, wantStatus: http.StatusMethodNotAllowed},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := performRequest(handler(), tc.method, "/health", "", nil)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%q", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantBody != "" && rec.Body.String() != tc.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
			if tc.wantStatus == http.StatusOK {
				if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
					t.Fatalf("Content-Type = %q, want text/plain; charset=utf-8", got)
				}
			}
		})
	}
}
