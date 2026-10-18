package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(serviceName).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var response healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if response.Service != serviceName {
		t.Fatalf("expected service %q, got %q", serviceName, response.Service)
	}
	if response.Status != "ok" {
		t.Fatalf("expected status ok, got %q", response.Status)
	}
}

func TestHealthListenerExposesFiniteScanOutcomes(t *testing.T) {
	// Use the same default registry as production's nil ScanMetrics fallback.
	// The service collector is initialized by grpcsvc, without starting a listener.
	rec := httptest.NewRecorder()
	healthHandler(serviceName).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status=%d body=%s", rec.Code, rec.Body.String())
	}
	var sampleLines int
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if strings.HasPrefix(line, "moderation_automod_scan_samples_total{") {
			sampleLines++
			valid := false
			for _, outcome := range []string{"acknowledged", "unconfirmed", "skipped_canceled", "skipped_budget"} {
				if strings.HasPrefix(line, "moderation_automod_scan_samples_total{outcome=\""+outcome+"\"} ") {
					valid = true
				}
			}
			if !valid {
				t.Fatalf("unexpected outcome or identity/content label: %s", line)
			}
		}
	}
	if sampleLines != 4 {
		t.Fatalf("expected four finite terminal outcome series, got%d", sampleLines)
	}
}

func TestHealthHandlerRejectsNonGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(serviceName).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
