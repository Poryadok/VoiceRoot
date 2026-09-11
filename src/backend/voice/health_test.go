package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(serviceName, nil).ServeHTTP(rec, req)

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

func TestHealthHandlerRejectsNonGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(serviceName, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestReadyHandlerSourceDisabled(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	healthHandler(serviceName, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("source-disabled readiness: expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestReadyHandlerFailsWhenVoiceSchemaCheckFails(t *testing.T) {
	readyErr := errors.New("voice lifecycle schema unavailable")
	checker := func(context.Context) error { return readyErr }

	readyReq := httptest.NewRequest(http.MethodGet, "/ready", nil)
	readyRec := httptest.NewRecorder()
	healthHandler(serviceName, checker).ServeHTTP(readyRec, readyReq)
	if readyRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("configured DB failure: expected status %d, got %d", http.StatusServiceUnavailable, readyRec.Code)
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthRec := httptest.NewRecorder()
	healthHandler(serviceName, checker).ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("DB failure must not change process liveness: expected status %d, got %d", http.StatusOK, healthRec.Code)
	}
}

func TestReadyHandlerBoundsVoiceSchemaCheck(t *testing.T) {
	checker := func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("configured Voice readiness check must have a deadline")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > 3*time.Second {
			t.Fatalf("configured Voice readiness deadline must be short and positive, got %s", remaining)
		}
		return nil
	}
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	healthHandler(serviceName, checker).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("healthy configured DB: expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestReadyHandlerRejectsNonGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ready", nil)
	rec := httptest.NewRecorder()

	healthHandler(serviceName, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
