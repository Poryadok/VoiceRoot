package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type failJSONWriter struct {
	header http.Header
}

func (f *failJSONWriter) Header() http.Header {
	if f.header == nil {
		f.header = make(http.Header)
	}
	return f.header
}

func (f *failJSONWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func (f *failJSONWriter) WriteHeader(int)           {}

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

func TestHealthHandlerRejectsNonGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(serviceName).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHealthHandlerEncodeError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthHandler(serviceName).ServeHTTP(&failJSONWriter{}, req)
}
