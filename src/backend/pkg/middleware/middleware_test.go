package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	var downstream string
	h := RequestID(func() string { return "gen-1" })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstream = r.Header.Get("X-Request-Id")
		w.WriteHeader(http.StatusNoContent)
	}))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("X-Request-Id"); got != "gen-1" {
		t.Fatalf("response X-Request-Id = %q, want gen-1", got)
	}
	if downstream != "gen-1" {
		t.Fatalf("handler saw X-Request-Id = %q", downstream)
	}
}

func TestRequestID_PreservesClientHeader(t *testing.T) {
	var downstream string
	h := RequestID(func() string { t.Fatal("generate should not run"); return "" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			downstream = r.Header.Get("X-Request-Id")
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("X-Request-Id", "client-id")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("X-Request-Id"); got != "client-id" {
		t.Fatalf("response X-Request-Id = %q", got)
	}
	if downstream != "client-id" {
		t.Fatalf("handler saw %q", downstream)
	}
}

func TestAccessLog_StatusCaptured(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{}))

	h := AccessLog(logger, "X-Request-Id", nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	out := buf.String()
	if !strings.Contains(out, `"status":418`) && !strings.Contains(out, `"status": 418`) {
		t.Fatalf("expected status 418 in log, got %q", out)
	}
}

func TestAccessLog_PreservesHijacker(t *testing.T) {
	var sawHijacker bool
	h := AccessLog(nil, "X-Request-Id", nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, sawHijacker = w.(http.Hijacker)
		w.WriteHeader(http.StatusNoContent)
	}))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if !sawHijacker {
		t.Fatal("AccessLog responseWriter must implement http.Hijacker for WebSocket proxying")
	}
}

func TestAccessLog_WithExtras(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{}))

	extras := func(r *http.Request) []slog.Attr {
		return []slog.Attr{slog.String("route_group", "alpha"), slog.String("remote_addr", r.RemoteAddr)}
	}
	h := AccessLog(logger, "X-Request-Id", extras)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	out := buf.String()
	if !strings.Contains(out, `"route_group":"alpha"`) && !strings.Contains(out, `"route_group": "alpha"`) {
		t.Fatalf("expected route_group in log, got %q", out)
	}
}
